// Ore → ingot planning for Smithing. Mechanics checked against live crafts and XP history:
// every swing adds (tool power + skill power) progress and grants
// ceil(xpPerProgress × progress × experienceRate) XP; smelting molten metal gives no XP.
// The backend serves the recipe chains and the player's setup; rows are recomputed here
// on every XP tick.

export const SMITHING_SKILL_ID = 6

const STORAGE_PREFIX = 'bitdash:smithing-ore:'

export function readCachedOreData(key) {
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + key)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

export function writeCachedOreData(key, data) {
  try {
    localStorage.setItem(STORAGE_PREFIX + key, JSON.stringify(data))
  } catch {
    // Storage full or blocked: the tab still works, it just refetches next time.
  }
}

// Asks this far below the median order are dumps or typos, not the price ingots sell at.
const OUTLIER_FACTOR = 3

// Price to list ingots at: the cheapest ask, skipping undercuts far below the usual asks.
// With no asks at all, the best bid is the only price on the market.
export function listingPrice(book) {
  const asks = book?.sell ?? []
  if (asks.length) {
    const medianAsk = asks[Math.floor((asks.length - 1) / 2)][0]
    return { price: asks.find(([price]) => price * OUTLIER_FACTOR >= medianAsk)[0], fromBids: false }
  }
  const bid = book?.buy?.[0]?.[0]
  return bid ? { price: bid, fromBids: true } : null
}

// What the ingots fetch right away from standing buy orders.
export function sellValue(book, quantity) {
  let left = quantity
  let value = 0
  for (const [price, qty] of book?.buy ?? []) {
    const take = Math.min(left, qty)
    value += take * price
    left -= take
    if (left <= 0) break
  }
  return { value, sold: quantity - left }
}

// Sums the per-tier ore counts of the tracked inventories.
export function trackedStock(inventories, trackedIds) {
  const tracked = new Set((trackedIds ?? []).map(String))
  const total = {}
  for (const inv of inventories ?? []) {
    if (!tracked.has(String(inv.id))) continue
    for (const [tier, counts] of Object.entries(inv.counts ?? {})) {
      total[tier] ??= { chunks: 0, packages: 0, pieces: 0, concentrates: 0, molten: 0 }
      for (const [kind, n] of Object.entries(counts)) total[tier][kind] = (total[tier][kind] ?? 0) + n
    }
  }
  return total
}

export function orePlanRow(chain, { xpLeft, setup, level, market, stock }) {
  const progress = setup.progressPerSwing
  const swingXp = Math.ceil(chain.xpPerProgress * progress * setup.xpRate)
  const xpConcentrate = progress > 0 ? (chain.concentrateEffort / progress) * swingXp : 0
  const xpForge = progress > 0 ? (chain.forgeEffort / progress) * swingXp : 0
  const xpPerIngot = chain.concentratesPerIngot * xpConcentrate + xpForge
  const effortPerIngot = chain.concentratesPerIngot * chain.concentrateEffort + chain.forgeEffort
  const ingots = xpPerIngot > 0 ? Math.ceil(Math.max(0, xpLeft) / xpPerIngot) : 0
  const piecesPerIngot = chain.concentratesPerIngot * chain.piecesPerConcentrate
  const swings = progress > 0 ? Math.ceil((ingots * effortPerIngot) / progress) : 0
  const lockedByLevel = level < chain.levelRequired
  const lockedByTool = (setup.tool?.level ?? 0) < chain.toolLevelRequired

  // Gathered ore counts for the XP it still has to give: raw ore for the whole chain,
  // concentrates and molten metal only for the steps left after them.
  const have = stock ?? {}
  const rawPieces = (have.chunks ?? 0) * chain.piecesPerChunk
    + (have.packages ?? 0) * (chain.piecesPerPackage ?? 0)
    + (have.pieces ?? 0)
  const stockXp = (piecesPerIngot ? (rawPieces / piecesPerIngot) * xpPerIngot : 0)
    + (have.concentrates ?? 0) * (xpForge / chain.concentratesPerIngot)
    + (have.molten ?? 0) * xpForge
  const xpStillNeeded = Math.max(0, xpLeft - stockXp)
  const piecesNeeded = xpPerIngot > 0 ? Math.ceil((xpStillNeeded / xpPerIngot) * piecesPerIngot) : 0
  const chunks = chain.piecesPerChunk ? Math.ceil(piecesNeeded / chain.piecesPerChunk) : null
  const chunkEquivalent = (xp) => (xpPerIngot > 0 && chain.piecesPerChunk ? (xp / xpPerIngot) * piecesPerIngot / chain.piecesPerChunk : 0)
  const book = market?.[`item:${chain.ingotId}`]
  const listing = listingPrice(book)
  return {
    ...chain,
    ingotPrice: listing?.price ?? null,
    priceFromBids: Boolean(listing?.fromBids),
    money: listing ? ingots * listing.price : null,
    instantSale: sellValue(book, ingots),
    swingXp,
    xpPerIngot,
    xpPerPiece: piecesPerIngot ? xpPerIngot / piecesPerIngot : 0,
    piecesPerIngot,
    ingots,
    pieces: piecesNeeded,
    chunks,
    chunksTotal: chunkEquivalent(Math.max(0, xpLeft)),
    haveChunks: chunkEquivalent(Math.min(stockXp, Math.max(0, xpLeft))),
    stock: have,
    flux: ingots * chain.fluxPerIngot,
    swings,
    seconds: swings * (chain.swingTime / (setup.speed || 1)),
    locked: lockedByLevel || lockedByTool,
    lockedByLevel,
    lockedByTool,
  }
}

// Fewest chunks still to gather first; tiers the player cannot craft yet go last.
export function orePlan(chains, { xpLeft, setup, level, market, stock }) {
  return chains
    .filter(Boolean)
    .map((chain) => orePlanRow(chain, { xpLeft, setup, level, market, stock: stock?.[String(chain.tier)] }))
    .sort((a, b) => (a.locked - b.locked) || ((a.chunks ?? Infinity) - (b.chunks ?? Infinity)) || (b.tier - a.tier))
}
