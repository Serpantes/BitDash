import { claimOf } from './marketTrades'
import { formatNumber, intlLocale, t } from './i18n'

export const DEVIATION_PCT = 10
export const FLAG_PCT = 15

function itemTypeOf(value) {
  return value === 1 || value === '1' || value === 'cargo' ? 1 : 0
}

export function marketReference(stats, side = 'sell') {
  const none = { value: null, kind: '', short: '', isAverage: false }
  if (!stats || typeof stats !== 'object') return none
  const book = side === 'buy'
    ? ['highestBuyPrice', t('marketRef.bestBuy'), t('marketRef.bestBuyShort')]
    : ['lowestSellPrice', t('marketRef.minSell'), t('marketRef.minSellShort')]
  const candidates = [
    ['vwap7d', t('marketRef.avg7d'), t('marketRef.avgShort')],
    ['vwap24h', t('marketRef.avg24h'), t('marketRef.avgShort')],
    book,
  ]
  for (const [key, kind, short] of candidates) {
    const n = Number(stats[key])
    if (Number.isFinite(n) && n > 0) return { value: n, kind, short, isAverage: key !== book[0] }
  }
  return none
}

export function marketPrices(stats, unitPrice) {
  const num = (value) => {
    const n = Number(value)
    return value != null && Number.isFinite(n) && n > 0 ? n : null
  }
  const lowest = num(stats?.lowestSellPrice)
  const avg7 = num(stats?.vwap7d)
  const average = avg7 ?? num(stats?.vwap24h)
  const pct = (base) => (base && unitPrice ? ((unitPrice - base) / base) * 100 : null)
  return {
    lowest,
    isLowest: lowest != null && unitPrice > 0 && unitPrice <= lowest,
    lowestPct: pct(lowest),
    average,
    averageWindow: average == null ? null : avg7 != null ? '7d' : '24h',
    averagePct: pct(average),
  }
}

export function playerOrderRows(orders, side, priceMaps, resolve) {
  return (orders ?? []).map((order, index) => {
    const itemType = itemTypeOf(order.itemType ?? order.item_type)
    const id = String(order.itemId ?? order.item_id ?? '')
    const known = resolve?.({ itemId: id, itemType }) ?? {}
    const quantity = Number(order.quantity) || 0
    const unitPrice = Number(order.priceThreshold ?? order.price) || 0
    const stats = (itemType === 1 ? priceMaps?.cargo : priceMaps?.items)?.[id]
    const reference = marketReference(stats, side)
    const diffPct = reference.value && unitPrice ? ((unitPrice - reference.value) / reference.value) * 100 : null
    return {
      key: String(order.entityId ?? `${side}-${index}`),
      side,
      good: {
        id,
        itemType,
        name: order.itemName ?? known.name ?? `#${id}`,
        iconAssetName: order.iconAssetName ?? known.iconAssetName ?? null,
        tier: order.tier ?? known.tier ?? null,
        rarityStr: order.rarityStr ?? known.rarityStr ?? null,
        tag: order.tag ?? null,
      },
      quantity,
      unitPrice,
      total: unitPrice * quantity,
      reference,
      market: marketPrices(stats, unitPrice),
      diffPct: Number.isFinite(diffPct) ? diffPct : null,
      claim: claimOf(order),
    }
  })
}

export function priceStanding(row) {
  if (row?.diffPct == null) return null
  if (Math.abs(row.diffPct) <= DEVIATION_PCT) return 'fair'
  return (row.side === 'buy') === (row.diffPct > 0) ? 'good' : 'bad'
}

export function standingHint(row) {
  if (row?.diffPct == null) return t('standing.noAverage')
  const pct = formatPct(Math.abs(row.diffPct)).replace('+', '')
  const kind = row.reference.kind
  const basis = `${kind.charAt(0).toUpperCase()}${kind.slice(1)}: ${formatPrice(row.reference.value)}`
  const standing = priceStanding(row)
  let text = t('standing.fair', { pct: DEVIATION_PCT })
  if (standing !== 'fair' && row.side === 'buy') {
    text = row.diffPct > 0 ? t('standing.buyAbove', { pct }) : t('standing.buyBelow', { pct })
  } else if (standing !== 'fair') {
    text = row.diffPct > 0 ? t('standing.sellAbove', { pct }) : t('standing.sellBelow', { pct })
  }
  return `${text}. ${basis}`
}

export function groupByClaim(rows) {
  const groups = new Map()
  for (const row of rows) {
    const key = row.claim.key || ''
    if (!groups.has(key)) groups.set(key, { key, claim: row.claim, total: 0, rows: [] })
    const group = groups.get(key)
    group.total += row.total
    group.rows.push(row)
  }
  for (const group of groups.values()) {
    group.rows.sort((a, b) => a.good.name.localeCompare(b.good.name) || b.total - a.total)
  }
  return [...groups.values()].sort((a, b) => b.total - a.total)
}

export function orderTotals(rows) {
  let total = 0
  let atAverage = 0
  let priced = 0
  const claims = new Set()
  for (const row of rows) {
    total += row.total
    if (row.reference.value) {
      atAverage += row.reference.value * row.quantity
      priced += 1
    }
    claims.add(row.claim.key)
  }
  return { count: rows.length, total, atAverage, priced, claims: claims.size }
}

export function bookRows(orders, side, playerEntityId) {
  return (orders ?? [])
    .map((order, index) => ({
      key: String(order.entityId ?? `${side}-${index}`),
      price: Number(order.priceThreshold) || 0,
      quantity: Number(order.quantity) || 0,
      owner: order.ownerUsername ?? null,
      mine: playerEntityId != null && String(order.ownerEntityId) === String(playerEntityId),
      claim: claimOf(order),
    }))
    .sort((a, b) => (side === 'buy' ? b.price - a.price : a.price - b.price) || b.quantity - a.quantity)
}

export function bookPlaces(rows, side) {
  return rows
    .filter((row) => row.mine)
    .map((row) => ({
      row,
      place: 1 + rows.filter((other) => (side === 'buy' ? other.price > row.price : other.price < row.price)).length,
      of: rows.length,
    }))
}

export function parseBitjitaTime(value) {
  if (!value) return null
  const iso = String(value).trim()
    .replace(' ', 'T')
    .replace(/(\.\d{3})\d+/, '$1')
    .replace(/([+-]\d{2})$/, '$1:00')
  const ms = Date.parse(iso)
  return Number.isFinite(ms) ? ms : null
}

export function priceSeries(priceData) {
  return (priceData ?? [])
    .map((bucket) => ({
      t: parseBitjitaTime(bucket.bucket),
      price: Number(bucket.avgPrice),
      volume: Number(bucket.volume) || 0,
      trades: Number(bucket.tradeCount) || 0,
    }))
    .filter((point) => point.t != null && Number.isFinite(point.price) && point.price > 0)
    .sort((a, b) => a.t - b.t)
}

export function recentTradeRows(trades) {
  return (trades ?? [])
    .map((trade, index) => ({
      key: String(trade.id ?? index),
      at: parseBitjitaTime(trade.timestamp),
      unitPrice: Number(trade.unitPrice) || 0,
      quantity: Number(trade.quantity) || 0,
      regionName: trade.regionName ?? null,
      seller: trade.sellerUsername ?? null,
      buyer: trade.buyerUsername ?? null,
    }))
    .sort((a, b) => (b.at ?? 0) - (a.at ?? 0))
}

export function formatAmount(value) {
  const n = Number(value)
  return value == null || value === '' || !Number.isFinite(n) ? '—' : formatNumber(Math.round(n), { maximumFractionDigits: 0 })
}

export function formatPrice(value) {
  const n = Number(value)
  if (value == null || value === '' || !Number.isFinite(n)) return '—'
  const digits = Math.abs(n) >= 1000 ? 0 : Math.abs(n) >= 100 ? 1 : 2
  return formatNumber(n, { maximumFractionDigits: digits })
}

export function formatPct(pct) {
  if (pct == null || !Number.isFinite(pct)) return '—'
  const abs = Math.abs(pct)
  const sign = pct > 0 ? '+' : pct < 0 ? '−' : ''
  return `${sign}${formatNumber(abs, { maximumFractionDigits: abs < 10 ? 1 : 0 })}%`
}

export function formatAgo(ms, now = Date.now()) {
  if (!ms) return '—'
  const minutes = Math.floor((now - ms) / 60000)
  if (minutes < 1) return t('ago.justNow')
  if (minutes < 60) return t('ago.minutes', { n: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('ago.hours', { n: hours })
  return new Date(ms).toLocaleString(intlLocale(), { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}
