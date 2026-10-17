export const COIN_ITEM_ID = 1

export function isCoinPile(pile) {
  return Number(pile?.itemId ?? pile?.itemStack?.item_id) === COIN_ITEM_ID || /coin/i.test(pile?.tag ?? '')
}

export function carriedCoins(inventories, playerEntityId) {
  return (inventories ?? [])
    .filter((inv) => String(inv?.ownerEntityId) === String(playerEntityId)
      && (inv?.playerOwnerEntityId == null || String(inv.playerOwnerEntityId) === '0'))
    .flatMap((inv) => inv.pockets ?? [])
    .reduce((sum, pocket) => {
      const contents = pocket?.contents
      const isCoins = Number(contents?.itemId) === COIN_ITEM_ID && Number(contents?.itemType ?? 0) === 0
      return isCoins ? sum + (Number(contents.quantity) || 0) : sum
    }, 0)
}

function pileAmount(pile) {
  return Number(pile?.quantity ?? pile?.itemStack?.quantity ?? 0) || 0
}

function msFromMicros(value) {
  const micros = Number(value)
  return Number.isFinite(micros) && micros > 0 ? Math.round(micros / 1000) : null
}

export function claimOf(row) {
  const x = Number(row?.claimLocationX)
  const z = Number(row?.claimLocationZ)
  return {
    key: String(row?.claimEntityId ?? row?.claimName ?? ''),
    name: row?.claimName ?? null,
    regionName: row?.regionName ?? null,
    x: Number.isFinite(x) ? x : null,
    z: Number.isFinite(z) ? z : null,
  }
}

function itemOf(row) {
  const id = row?.itemId ?? row?.itemStack?.item_id ?? null
  return {
    id: id == null ? null : String(id),
    name: row?.itemName ?? null,
    iconAssetName: row?.iconAssetName ?? null,
    itemType: Number(row?.itemType) === 1 ? 1 : 0,
    tier: row?.tier ?? row?.itemTier ?? null,
    rarityStr: row?.rarityStr ?? row?.itemRarityStr ?? null,
    tag: row?.tag ?? null,
  }
}

function tradeOf(trade, playerEntityId) {
  const sale = String(trade.sellerEntityId) === String(playerEntityId)
  return {
    key: trade.id ?? `${trade.orderEntityId}-${trade.timestamp}`,
    side: sale ? 'sell' : 'buy',
    orderEntityId: trade.orderEntityId ?? null,
    quantity: Number(trade.quantity) || 0,
    unitPrice: Number(trade.unitPrice) || 0,
    totalPrice: Number(trade.totalPrice) || 0,
    counterparty: (sale ? trade.purchaserUsername : trade.sellerUsername) ?? null,
    item: itemOf(trade),
    claim: claimOf(trade),
    at: msFromMicros(trade.timestamp),
  }
}

export function collectRows(collections, trades, playerEntityId) {
  const byTimestamp = new Map()
  ;(trades ?? []).forEach((trade) => {
    const key = String(trade.timestamp)
    if (!byTimestamp.has(key)) byTimestamp.set(key, [])
    byTimestamp.get(key).push(trade)
  })
  const taken = new Set()

  return (collections ?? [])
    .slice()
    .sort((a, b) => Number(b.timestamp ?? 0) - Number(a.timestamp ?? 0))
    .map((pile, index) => {
      const isCoins = isCoinPile(pile)
      const amount = pileAmount(pile)
      const candidates = (byTimestamp.get(String(pile.timestamp)) ?? []).filter((t) => !taken.has(t.id))
      const raw = candidates.find((t) => Number(isCoins ? t.totalPrice : t.quantity) === amount) ?? null
      if (raw) taken.add(raw.id)
      const trade = raw ? tradeOf(raw, playerEntityId) : null
      return {
        key: String(pile.entityId ?? `${pile.timestamp}-${index}`),
        isCoins,
        amount,
        at: msFromMicros(pile.timestamp),
        claim: claimOf(pile),
        item: trade?.item ?? itemOf(pile),
        trade,
      }
    })
}

export function collectGroups(rows) {
  const groups = new Map()
  rows.forEach((row) => {
    const key = row.claim.key || ''
    if (!groups.has(key)) groups.set(key, { key, claim: row.claim, coins: 0, items: 0, rows: [] })
    const group = groups.get(key)
    if (row.isCoins) group.coins += row.amount
    else group.items += 1
    group.rows.push(row)
  })
  return [...groups.values()].sort((a, b) => b.coins - a.coins || b.rows.length - a.rows.length)
}

export function collectSummary(rows) {
  const coinRows = rows.filter((row) => row.isCoins)
  return {
    coins: coinRows.reduce((sum, row) => sum + row.amount, 0),
    piles: rows.length,
    coinPiles: coinRows.length,
    itemPiles: rows.length - coinRows.length,
    claims: new Set(rows.map((row) => row.claim.key)).size,
    linked: rows.filter((row) => row.trade).length,
  }
}

export function tradeRows(trades, playerEntityId) {
  return (trades ?? [])
    .map((trade) => tradeOf(trade, playerEntityId))
    .sort((a, b) => (b.at ?? 0) - (a.at ?? 0))
}

export function tradeTotals(rows, sinceMs) {
  const recent = rows.filter((row) => row.at != null && row.at >= sinceMs)
  const sales = recent.filter((row) => row.side === 'sell')
  const purchases = recent.filter((row) => row.side === 'buy')
  return {
    sales: sales.length,
    purchases: purchases.length,
    sold: sales.reduce((sum, row) => sum + row.totalPrice, 0),
    bought: purchases.reduce((sum, row) => sum + row.totalPrice, 0),
  }
}
