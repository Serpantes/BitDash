// Ore → ingot planning for Smithing. Mechanics checked against live crafts and XP history:
// every swing adds (tool power + skill power) progress and grants
// ceil(xpPerProgress × progress × experienceRate) XP; smelting molten metal gives no XP.
// The backend serves the recipe chains and the player's setup; rows are recomputed here
// on every XP tick.

export const SMITHING_SKILL_ID = 6

const STORAGE_PREFIX = 'bitdash:smithing-ore:'

export function readCachedOreData(playerEntityId) {
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + playerEntityId)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

export function writeCachedOreData(playerEntityId, data) {
  try {
    localStorage.setItem(STORAGE_PREFIX + playerEntityId, JSON.stringify(data))
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

export function orePlanRow(chain, { xpLeft, setup, level, market }) {
  const progress = setup.progressPerSwing
  const swingXp = Math.ceil(chain.xpPerProgress * progress * setup.xpRate)
  const effortPerIngot = chain.concentratesPerIngot * chain.concentrateEffort + chain.forgeEffort
  const xpPerIngot = progress > 0 ? (effortPerIngot / progress) * swingXp : 0
  const ingots = xpPerIngot > 0 ? Math.ceil(Math.max(0, xpLeft) / xpPerIngot) : 0
  const piecesPerIngot = chain.concentratesPerIngot * chain.piecesPerConcentrate
  const swings = progress > 0 ? Math.ceil((ingots * effortPerIngot) / progress) : 0
  const lockedByLevel = level < chain.levelRequired
  const lockedByTool = (setup.tool?.level ?? 0) < chain.toolLevelRequired
  const chunks = chain.piecesPerChunk ? Math.ceil((ingots * piecesPerIngot) / chain.piecesPerChunk) : null
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
    pieces: ingots * piecesPerIngot,
    chunks,
    flux: ingots * chain.fluxPerIngot,
    swings,
    seconds: swings * (chain.swingTime / (setup.speed || 1)),
    locked: lockedByLevel || lockedByTool,
    lockedByLevel,
    lockedByTool,
  }
}

// Fewest chunks first; tiers the player cannot craft yet go last.
export function orePlan(chains, { xpLeft, setup, level, market }) {
  return chains
    .filter(Boolean)
    .map((chain) => orePlanRow(chain, { xpLeft, setup, level, market }))
    .sort((a, b) => (a.locked - b.locked) || ((a.chunks ?? Infinity) - (b.chunks ?? Infinity)) || (b.tier - a.tier))
}
