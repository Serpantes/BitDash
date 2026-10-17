import { test, expect } from '@playwright/test'

const PLAYER_ID = '1224979098660014211'

function mapBlock(mapState) {
  return {
    blockType: 'map',
    sourceType: 'map',
    inventoryId: 'map:e2e',
    inventoryName: 'Map',
    mapState,
    mapTileSize: 'large',
  }
}

async function openMap(page, mapState = {}) {
  await page.addInitScript((block) => {
    localStorage.setItem('bitdash.locale', 'en')
    if (!localStorage.getItem('bitdash:pinned')) localStorage.setItem('bitdash:pinned', JSON.stringify([block]))
  }, mapBlock(mapState))
  await page.goto('/')
  await expect(page.locator('.map-tile')).toBeVisible()
  await expect(page.locator('.map-veil')).toHaveCount(0, { timeout: 60_000 })
  return page.frames().find((frame) => frame.url().includes('/map-embed/'))
}

function savedHash(page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('bitdash:pinned'))[0].mapState?.hash ?? null)
}

test.describe('map tile', () => {
  test('shows the bitjita map with its controls but without the site header', async ({ page }) => {
    const frame = await openMap(page)
    await expect(frame.locator('[role="application"] canvas')).toBeVisible()
    await expect(frame.getByRole('button', { name: 'Search the map' })).toBeVisible()
    await expect(frame.locator('header').first()).toBeHidden()
  })

  test('keeps the view and tracked entities across reloads', async ({ page }) => {
    const frame = await openMap(page, { hash: '2/5000/5000' })
    await frame.evaluate(() => { location.hash = '3/4100/4200&t=mob:7' })
    await expect.poll(() => savedHash(page)).toContain('t=mob:7')

    await page.reload()
    await expect(page.locator('.map-veil')).toHaveCount(0, { timeout: 60_000 })
    await expect(page.locator('.map-frame')).toHaveAttribute('src', /#3\/4100\/4200&t=mob:7/)
  })

  test('carries a bitcraftmap tile over to bitjita', async ({ page }) => {
    await openMap(page, { query: `?playerId=${PLAYER_ID}&enemyId=7,8&followPlayer=1`, view: { n: 3000, e: 4000, zoom: 2 } })
    await expect(page.locator('.map-frame')).toHaveAttribute('src', `/map-embed/map#0/3000/4000&t=mob:7,mob:8&pid=${PLAYER_ID}&follow=1`)
  })

  test('does not fly to a watched player on load', async ({ page }) => {
    const frame = await openMap(page, { hash: `2/5000/5000&pid=${PLAYER_ID}` })
    await page.waitForTimeout(8000)
    expect(frame.url()).toContain('#2/5000/5000&')
  })

  test('finds players from the map search', async ({ page }) => {
    const frame = await openMap(page, { hash: '2/5000/5000' })
    await frame.getByRole('button', { name: 'Search the map' }).click()
    await frame.locator('input[type=search], input[placeholder*="earch"]').first().pressSequentially('Serpantes', { delay: 60 })
    const option = frame.locator('[role=option]').filter({ hasText: 'Serpantes' }).first()
    await expect(option).toBeVisible({ timeout: 10_000 })

    await option.click()
    await expect.poll(() => savedHash(page)).toContain(`pid=${PLAYER_ID}`)
    expect(await savedHash(page)).toMatch(/^2\/5000\/5000&/)
  })
})
