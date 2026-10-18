import { test, expect } from '@playwright/test'

const PLAYER_ID = '1224979098660014211'
const CLAIM = { claimEntityId: '501', claimName: 'CSB Aurora', regionName: 'Elyndor', claimLocationX: 11407, claimLocationZ: 17457 }

function order(entityId, itemId, itemName, priceThreshold, quantity, extra = {}) {
  return { entityId, itemId, itemType: 0, itemName, priceThreshold: String(priceThreshold), quantity: String(quantity), tier: 7, tag: 'Ingot', ...CLAIM, ...extra }
}

const SELL_ORDERS = [
  order('1', '1203417340', 'Aurumite Hammer', 40000, 4, { tag: 'Blacksmith Tool', rarityStr: 'Epic' }),
  order('2', '2001', 'Aurumite Ingot', 120, 700),
  order('3', '2002', 'Rathium Ingot', 115, 1, { claimEntityId: '502', claimName: 'MATCHA LATTE' }),
]

const PRICES = {
  1203417340: { lowestSellPrice: 40000, highestBuyPrice: 1, vwap7d: 40000, vwap24h: null },
  2001: { lowestSellPrice: 100, highestBuyPrice: 90, vwap7d: 100, vwap24h: 105 },
  2002: { lowestSellPrice: 115, highestBuyPrice: null, vwap7d: null, vwap24h: null },
}

const TRADE = {
  id: 't1',
  itemId: '2003',
  itemType: 0,
  itemName: 'Emarium Ingot',
  quantity: 220,
  unitPrice: 74,
  totalPrice: 16280,
  sellerEntityId: PLAYER_ID,
  sellerUsername: 'Serpantes',
  purchaserUsername: 'BigKitten',
  timestamp: String((Date.now() - 3600_000) * 1000),
  claimName: 'CSB Aurora',
}

const HAMMER_DETAIL = {
  item: { id: '1203417340', name: 'Aurumite Hammer', tier: 7, rarityStr: 'Epic', tag: 'Blacksmith Tool' },
  sellOrders: [
    { entityId: '1', priceThreshold: '40000', quantity: '4', ownerEntityId: PLAYER_ID, ownerUsername: 'Serpantes', ...CLAIM },
    { entityId: '9', priceThreshold: '45000', quantity: '2', ownerEntityId: '7', ownerUsername: 'Other', claimName: 'Lethe', regionName: 'Draxen' },
  ],
  buyOrders: [{ entityId: '10', priceThreshold: '1', quantity: '1', ownerEntityId: '8', ownerUsername: 'Cheap', claimName: 'Xeltrassan Nexus', regionName: 'Marowik' }],
  stats: { lowestSell: 40000, highestBuy: 1, sellOrderCount: 2, buyOrderCount: 1, totalAvailableSell: 6, totalAvailableBuy: 1 },
}

function json(route, body) {
  return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockBitjita(page) {
  await page.route('**/api/bitjita/**', (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname.replace('/api/bitjita', '')
    if (path === `/players/${PLAYER_ID}/market`) return json(route, { sellOrders: SELL_ORDERS, buyOrders: [] })
    if (path === `/players/${PLAYER_ID}/market-collections`) return json(route, { collections: [] })
    if (path === `/players/${PLAYER_ID}/inventories`) return json(route, { inventories: [] })
    if (path === `/market/player/${PLAYER_ID}/trades`) return json(route, { trades: [TRADE], total: 1 })
    if (path === '/market/prices/bulk') {
      const ids = route.request().postDataJSON()?.itemIds ?? []
      return json(route, { data: { items: Object.fromEntries(ids.filter((id) => PRICES[id]).map((id) => [id, PRICES[id]])), cargo: {} } })
    }
    if (path === '/market/item/1203417340') return json(route, HAMMER_DETAIL)
    if (path === '/market/item/1203417340/price-history') return json(route, { priceData: [], recentTrades: [], priceStats: {} })
    return route.continue()
  })
}

async function openMarket(page, { size = 'large', tab } = {}) {
  await mockBitjita(page)
  const block = {
    blockType: 'market',
    sourceType: 'market',
    inventoryId: `market:${PLAYER_ID}`,
    playerEntityId: PLAYER_ID,
    playerName: 'Serpantes',
    inventoryName: 'Market · Serpantes',
    marketTileSize: size,
    ...(tab ? { marketTab: tab } : {}),
  }
  await page.addInitScript((pinned) => {
    localStorage.setItem('bitdash.locale', 'en')
    localStorage.setItem('bitdash:pinned', JSON.stringify([pinned]))
  }, block)
  await page.goto('/')
  const market = page.locator('.market')
  await expect(market).toBeVisible()
  return market
}

function orderRow(market, name) {
  return market.locator('button.mrow').filter({ hasText: name })
}

test.describe('market block', () => {
  test('labels every price column on a large tile', async ({ page }) => {
    const market = await openMarket(page)
    const head = market.locator('.mrow--head')
    for (const label of ['Item', 'Your price', 'Lowest now', '7d average', 'Total']) {
      await expect(head.getByText(label, { exact: true })).toBeVisible()
    }

    const hammer = orderRow(market, 'Aurumite Hammer')
    await expect(hammer).toContainText('40,000')
    await expect(hammer).toContainText('per unit')
    await expect(hammer.locator('.mrow-tag--narrow')).toBeHidden()
    await expect(hammer.locator('.mrow-col--wide .mrow-tag--good')).toHaveText("that's you")
    await expect(hammer).toContainText('= average')
    await expect(hammer).toContainText('160,000')
    await expect(hammer).toContainText('4 × 40,000')

    const ingot = orderRow(market, 'Aurumite Ingot')
    await expect(ingot.locator('.mrow-col--wide .mrow-tag--warn')).toHaveText("you're 20% higher")
    await expect(ingot.locator('.mrow-tag--bad')).toHaveText('▲ 20%')
    await expect(ingot).toContainText('84,000')

    await expect(orderRow(market, 'Rathium Ingot')).toContainText('no trades')
    await expect(market.locator('.market-foot')).toContainText('3 lots · 2 markets')
    await expect(market).not.toContainText(/\b(common|marketBlock|marketItem)\.\w+/)
  })

  test('keeps the small tile compact', async ({ page }) => {
    const market = await openMarket(page, { size: 'small' })
    const head = market.locator('.mrow--head')
    await expect(head.getByText('Your price', { exact: true })).toBeVisible()
    await expect(head.getByText('Lowest now', { exact: true })).toBeHidden()

    const ingot = orderRow(market, 'Aurumite Ingot')
    await expect(ingot.locator('.mrow-col--wide').first()).toBeHidden()
    await expect(ingot.locator('.mrow-tag--narrow')).toHaveText('lowest 100')
    await expect(ingot).toContainText('total 84,000 · avg 100')
    await expect(orderRow(market, 'Aurumite Hammer').locator('.mrow-tag--narrow')).toHaveText("that's you")
  })

  test('spells out quantity and price in completed trades', async ({ page }) => {
    const market = await openMarket(page, { tab: 'trades' })
    await expect(market.locator('.mrow--trade').filter({ hasText: 'Emarium Ingot' })).toContainText('220 pcs at 74 · bought by BigKitten')
    await expect(market.locator('.market-foot')).toContainText('16,280')
  })

  test('ties the item summary to its order books', async ({ page }) => {
    const market = await openMarket(page)
    await orderRow(market, 'Aurumite Hammer').click()

    const panel = market.locator('.mi')
    await expect(panel.locator('.mi-kpi--sell')).toContainText('Sell from')
    await expect(panel.locator('.mi-kpi--sell')).toContainText('40,000')
    await expect(panel.locator('.mi-kpi--sell')).toContainText('2 orders · 6 pcs')
    await expect(panel.locator('.mi-kpi--buy')).toContainText('Buy up to')
    await expect(panel.locator('.mi-book--sell .mi-offer--best')).toHaveCount(1)
    await expect(panel.locator('.mi-book--sell .mi-offer--best')).toContainText('you')
    await expect(panel.locator('.mi-mine')).toContainText('best price of 2')
    await expect(panel).not.toContainText(/\b(common|marketBlock|marketItem)\.\w+/)
  })
})
