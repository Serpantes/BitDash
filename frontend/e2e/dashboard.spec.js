import { test, expect } from '@playwright/test'

function mapBlock(id) {
  return { blockType: 'map', sourceType: 'map', inventoryId: id, inventoryName: 'Map', mapState: { hash: '2/5000/5000' }, mapTileSize: 'small' }
}

function saveDashboard(page, blocks) {
  return page.request.put('/api/dashboard', { data: { force: true, blocks } })
}

test.describe('dashboard persistence', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem('bitdash.locale', 'en'))
  })

  test('restores blocks from the server when browser storage is empty', async ({ page }) => {
    await saveDashboard(page, [mapBlock('map:persist')])
    await page.addInitScript(() => localStorage.removeItem('bitdash:pinned'))
    await page.goto('/')
    await expect(page.locator('.map-tile')).toHaveCount(1)
    expect(await page.evaluate(() => JSON.parse(localStorage.getItem('bitdash:pinned')).map((b) => b.inventoryId))).toEqual(['map:persist'])
  })

  test('a stale tab does not overwrite blocks saved elsewhere', async ({ page }) => {
    await saveDashboard(page, [mapBlock('map:a')])
    await page.goto('/')
    await expect(page.locator('.map-tile')).toHaveCount(1)

    await saveDashboard(page, [mapBlock('map:a'), mapBlock('map:b')])
    await page.locator('.bc-close').first().click()

    await expect(page.locator('.map-tile')).toHaveCount(2)
    const saved = await (await page.request.get('/api/dashboard')).json()
    expect(saved.blocks.map((b) => b.inventoryId)).toEqual(['map:a', 'map:b'])
  })
})
