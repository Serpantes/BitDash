import { test, expect } from '@playwright/test'

const PLAYER_ID = '1224979098660014211'
const CLAIM = { claimEntityId: '501', claimName: 'CSB Aurora', regionId: 12, regionName: 'Elyndor', claimLocationX: 11407, claimLocationZ: 17457 }

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
    { entityId: '9', priceThreshold: '45000', quantity: '2', ownerEntityId: '7', ownerUsername: 'Other', claimName: 'Lethe', regionId: 12, regionName: 'Elyndor' },
  ],
  buyOrders: [{ entityId: '10', priceThreshold: '1', quantity: '1', ownerEntityId: '8', ownerUsername: 'Cheap', claimName: 'Xeltrassan Nexus', regionName: 'Marowik' }],
  stats: { lowestSell: 40000, highestBuy: 1, sellOrderCount: 2, buyOrderCount: 1, totalAvailableSell: 6, totalAvailableBuy: 1 },
}

function rival(entityId, ownerEntityId, ownerUsername, price, quantity, regionId, claimName) {
  return { entityId, ownerEntityId, ownerUsername, priceThreshold: String(price), quantity: String(quantity), regionId, claimName }
}

const BOOKS = {
  1203417340: HAMMER_DETAIL,
  2001: {
    sellOrders: [
      { entityId: '2', ownerEntityId: PLAYER_ID, ownerUsername: 'Serpantes', priceThreshold: '100', quantity: '50', ...CLAIM },
      rival('21', '31', 'Tamamo', 110, 952, 12, 'Freeport'),
      rival('22', '32', 'Bob', 115, 40, 12, 'Lethe'),
      rival('23', '33', 'Far', 90, 10, 5, 'Elsewhere'),
    ],
    buyOrders: [],
  },
  2002: { sellOrders: [{ entityId: '3', ownerEntityId: PLAYER_ID, priceThreshold: '115', quantity: '1', ...CLAIM }], buyOrders: [] },
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
    const book = path.match(/^\/market\/item\/(\d+)$/)
    if (book && BOOKS[book[1]]) return json(route, BOOKS[book[1]])
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
  await page.request.put('/api/dashboard', { data: { force: true, blocks: [block] } })
  await page.addInitScript(() => localStorage.setItem('bitdash.locale', 'en'))
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
    for (const label of ['Item', 'Your price', 'In region', '7d average', 'Total']) {
      await expect(head.getByText(label, { exact: true })).toBeVisible()
    }

    const hammer = orderRow(market, 'Aurumite Hammer')
    await expect(hammer).toContainText('40,000')
    await expect(hammer).toContainText('per unit')
    await expect(hammer.locator('.mrow-tag--narrow')).toBeHidden()
    const hammerRegion = hammer.locator('.mrow-col--wide').first()
    await expect(hammerRegion.locator('.mrow-value')).toHaveText('45,000')
    await expect(hammerRegion.locator('.mrow-tag')).toHaveText('1 seller')
    await expect(hammer).toContainText('= average')
    await expect(hammer).toContainText('160,000')
    await expect(hammer).toContainText('4 × 40,000')

    const ingot = orderRow(market, 'Aurumite Ingot')
    const ingotRegion = ingot.locator('.mrow-col--wide').first()
    await expect(ingotRegion.locator('.mrow-value')).toHaveText('110')
    await expect(ingotRegion.locator('.mrow-tag')).toHaveText('2 sellers')
    await expect(ingotRegion).toHaveAttribute('title', 'Elyndor: cheapest from Tamamo — 110 (×952, Freeport). 2 sellers in total, 992 pcs')
    await expect(ingot.locator('.mrow-tag--bad')).toHaveText('▲ 20%')
    await expect(ingot).toContainText('84,000')

    const rathium = orderRow(market, 'Rathium Ingot')
    await expect(rathium).toContainText('no trades')
    await expect(rathium.locator('.mrow-col--wide').first().locator('.mrow-tag')).toHaveText('only you')
    await expect(market.locator('.market-foot')).toContainText('3 lots · 2 markets')
    await expect(market).not.toContainText(/\b(common|marketBlock|marketItem)\.\w+/)
  })

  test('keeps the small tile compact', async ({ page }) => {
    const market = await openMarket(page, { size: 'small' })
    const head = market.locator('.mrow--head')
    await expect(head.getByText('Your price', { exact: true })).toBeVisible()
    await expect(head.getByText('In region', { exact: true })).toBeHidden()

    const ingot = orderRow(market, 'Aurumite Ingot')
    await expect(ingot.locator('.mrow-col--wide').first()).toBeHidden()
    await expect(ingot.locator('.mrow-tag--narrow')).toHaveText('region 110')
    await expect(ingot).toContainText('total 84,000 · avg 100')
    await expect(orderRow(market, 'Rathium Ingot').locator('.mrow-tag--narrow')).toHaveText('region: only you')
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
