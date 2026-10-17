import { test, expect } from '@playwright/test'

const MAP_BLOCK = {
  blockType: 'map',
  sourceType: 'map',
  inventoryId: 'map:e2e',
  inventoryName: 'Map',
  mapState: {},
  mapTileSize: 'large',
}

async function openMap(page) {
  await page.addInitScript((block) => {
    localStorage.setItem('bitdash.locale', 'en')
    localStorage.setItem('bitdash:pinned', JSON.stringify([block]))
  }, MAP_BLOCK)
  await page.goto('/')
  await expect(page.locator('.map-tile')).toBeVisible()
  await expect(page.locator('.map-veil')).toHaveCount(0, { timeout: 60_000 })
  return page.frameLocator('.map-frame')
}

async function search(page, query) {
  await page.fill('.map-search input', query)
  await expect(page.locator('.map-results-empty')).toHaveCount(0)
  await expect(page.locator('.map-result').first()).toBeVisible()
  // the bridge streams results while upstream keeps loading; let the list settle
  await page.waitForTimeout(4000)
}

async function setOnlinePlayers(page, on) {
  await page.click('.map-layers-btn')
  const row = page.locator('.map-toggle-row', { hasText: 'All online players' })
  const toggle = row.locator('.bc-toggle')
  await expect(toggle).toBeVisible()
  if ((await toggle.evaluate((el) => el.classList.contains('on'))) !== on) {
    await row.click()
    await expect(toggle).toHaveClass(on ? /\bon\b/ : /^(?!.*\bon\b)/)
  }
  await page.click('.map-layers-btn')
}

async function resultEntries(page) {
  return page.locator('.map-results').evaluate((panel) => {
    const entries = []
    let group = ''
    for (const el of panel.children) {
      if (el.classList.contains('map-panel-title')) group = el.textContent.trim()
      else if (el.classList.contains('map-result')) {
        entries.push({
          group,
          label: el.querySelector('.map-result-label').textContent.trim(),
          meta: el.querySelector('.map-result-meta').textContent.trim(),
        })
      }
    }
    return entries
  })
}

// Upstream labels like "Luminite Outcrop Interior" are prefixes of earlier rows such as
// "Luminite Outcrop Interior Depleted"; picking must track the row that was clicked.
function prefixShadowed(entries) {
  return entries.find((entry, i) => entries
    .slice(0, i)
    .some((other) => other.group === entry.group && other.label.startsWith(`${entry.label} `)))
}

test.describe('map tile', () => {
  test('tracks the clicked resource, not an earlier one with a similar name', async ({ page }) => {
    await openMap(page)
    await search(page, 'luminite')
    const target = prefixShadowed(await resultEntries(page))
    test.skip(!target, 'upstream returned no prefix-shadowed resources for this query')

    await page.locator('.map-result').filter({ has: page.getByText(target.label, { exact: true }) }).first().click()

    const chip = page.locator('.map-chip-label')
    await expect(chip).toHaveCount(1)
    await expect(chip).toHaveText(`${target.label} · ${target.meta}`)
  })

  test('tracks the keyboard-selected resource', async ({ page }) => {
    await openMap(page)
    await search(page, 'luminite')
    const entries = await resultEntries(page)
    const target = prefixShadowed(entries)
    test.skip(!target, 'upstream returned no prefix-shadowed resources for this query')

    const input = page.locator('.map-search input')
    for (let i = 0; i < entries.indexOf(target); i += 1) await input.press('ArrowDown')
    await expect(page.locator('.map-result.active .map-result-label')).toHaveText(target.label)
    await input.press('Enter')

    await expect(page.locator('.map-chip-label')).toHaveText(`${target.label} · ${target.meta}`)
  })

  // Upstream only reconnects its relay client after a connect error, so a clean close
  // (relay restart, idle timeout) used to freeze every tracked marker until a reload.
  test('reconnects the live relay after it closes cleanly', async ({ page }) => {
    const relays = []
    await page.context().routeWebSocket(/prism\.brico\.app/, (ws) => {
      ws.connectToServer()
      relays.push(ws)
    })
    await openMap(page)
    await search(page, 'luminite')
    const target = prefixShadowed(await resultEntries(page))
    test.skip(!target, 'upstream returned no trackable resources for this query')
    await page.locator('.map-result').filter({ has: page.getByText(target.label, { exact: true }) }).first().click()
    await expect(page.locator('.map-chip-label')).toHaveCount(1)
    await expect.poll(() => relays.length).toBeGreaterThan(0)

    const opened = relays.length
    await relays[opened - 1].close({ code: 1000, reason: 'relay restart' })
    await expect(page.locator('.map-relay')).toBeVisible()
    await expect.poll(() => relays.length, { timeout: 20_000 }).toBeGreaterThan(opened)
    await expect(page.locator('.map-relay')).toHaveCount(0)
  })

  test('shows player names on hover over the online players layer', async ({ page }) => {
    const frame = await openMap(page)
    await setOnlinePlayers(page, true)

    const point = await findPlayerPixel(page, frame)
    test.skip(!point, 'no online players drawn in the current view')

    const box = await page.locator('.map-frame').boundingBox()
    await page.mouse.move(box.x + point.x, box.y + point.y)
    const tip = frame.locator('.leaflet-tooltip.bitdash-hover-tip')
    await expect(tip).toBeVisible()
    await expect(tip).not.toHaveText('')

    await page.mouse.move(box.x + 5, box.y + box.height - 5)
    await expect(tip).toHaveCount(0)
  })

  test('shows the tracked player name on hover over its marker', async ({ page, browser }, testInfo) => {
    const frame = await openMap(page)
    await setOnlinePlayers(page, true)

    const point = await findPlayerPixel(page, frame)
    test.skip(!point, 'no online players drawn in the current view')

    const box = await page.locator('.map-frame').boundingBox()
    await page.mouse.move(box.x + point.x, box.y + point.y)
    const name = (await frame.locator('.leaflet-tooltip.bitdash-hover-tip').textContent()).trim()
    expect(name).not.toBe('')

    // once the online players layer has been on, upstream stops finding and tracking
    // players in that browser profile, so track from a clean one
    const { baseURL, viewport } = testInfo.project.use
    const context = await browser.newContext({ baseURL, viewport })
    const fresh = await context.newPage()
    const freshFrame = await openMap(fresh)
    await search(fresh, name)
    await fresh.locator('.map-result').filter({ has: fresh.getByText(name, { exact: true }) }).first().click()
    await expect(fresh.locator('.map-chip-label')).toHaveText(name)

    const marker = freshFrame.locator('.leaflet-markerOnTop-pane .leaflet-marker-icon').last()
    await expect(marker).toBeVisible()
    await marker.hover()
    await expect(freshFrame.locator('.leaflet-tooltip.bitdash-hover-tip')).toHaveText(name)
    await context.close()
  })
})

// Finds a drawn dot on the "All online players" canvas by its pixels.
async function findPlayerPixel(page, frame) {
  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    const point = await frame.locator('body').evaluate(() => {
      for (const canvas of document.querySelectorAll('.leaflet-container canvas')) {
        if (!canvas.width || !canvas.height) continue
        let data
        try {
          data = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data
        } catch {
          continue
        }
        const rect = canvas.getBoundingClientRect()
        const margin = 80
        for (let y = margin; y < canvas.height - margin; y += 2) {
          for (let x = margin; x < canvas.width - margin; x += 2) {
            if (data[(y * canvas.width + x) * 4 + 3] > 200) {
              return { x: rect.left + x, y: rect.top + y + 1 }
            }
          }
        }
      }
      return null
    })
    if (point) return point
    await page.waitForTimeout(1000)
  }
  return null
}
