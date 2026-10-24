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

// Asks far above the usual price are listings nobody fills; they are neither bought nor used
// to price the part of the need the market cannot cover.
const OUTLIER_FACTOR = 3

// Price at which half of the sensibly priced quantity is on sale. Outliers are cut against the
// median order first, so a few huge listings at absurd prices cannot drag it up.
export function typicalPrice(levels) {
  if (!levels.length) return null
  const medianOrder = levels[Math.floor((levels.length - 1) / 2)][0]
  const sane = levels.filter(([price]) => price <= medianOrder * OUTLIER_FACTOR)
  const total = sane.reduce((sum, [, qty]) => sum + qty, 0)
  let seen = 0
  for (const [price, qty] of sane) {
    seen += qty
    if (seen * 2 >= total) return price
  }
  return medianOrder
}

// Buys the sensible sell orders cheapest first; whatever the market cannot cover is priced at
// the typical price.
export function buyCost(book, quantity) {
  const levels = book?.sell ?? []
  if (!levels.length || quantity <= 0) return quantity > 0 ? null : { cost: 0, onSale: 0, partial: false }
  const typical = typicalPrice(levels)
  let left = quantity
  let cost = 0
  for (const [price, qty] of levels) {
    if (price > typical * OUTLIER_FACTOR) break
    const take = Math.min(left, qty)
    cost += take * price
    left -= take
    if (left <= 0) break
  }
  const onSale = quantity - left
  if (left > 0) cost += left * typical
  return { cost, onSale, partial: left > 0, typical }
}

// What the crafted ingots fetch right away from standing buy orders.
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

function marketPart(market, kind, id, quantity) {
  if (!id || quantity <= 0) return null
  const book = market?.[`${kind}:${id}`]
  return { quantity, lowest: book?.sell?.[0]?.[0] ?? null, ...(buyCost(book, quantity) ?? { cost: null, onSale: 0, partial: false }) }
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
  // High-tier chunks often have no sellers while their ore pieces do; price whichever is on sale.
  const chunkBuy = marketPart(market, 'cargo', chain.chunkCargoId, chunks ?? 0)
  const pieceBuy = marketPart(market, 'item', chain.pieceItemId, ingots * piecesPerIngot)
  const parts = {
    chunks: chunkBuy?.cost == null && pieceBuy?.cost != null ? { ...pieceBuy, asPieces: true } : chunkBuy,
    logs: marketPart(market, 'item', chain.logItemId, ingots),
    flux: marketPart(market, 'item', chain.fluxItemId, ingots * chain.fluxPerIngot),
  }
  const priced = Object.values(parts).filter((p) => p?.cost != null)
  return {
    ...chain,
    parts,
    money: parts.chunks?.cost != null ? priced.reduce((sum, p) => sum + p.cost, 0) : null,
    moneyPartial: priced.some((p) => p.partial) || Object.values(parts).some((p) => p && p.cost == null),
    ingotSale: sellValue(market?.[`item:${chain.ingotId}`], ingots),
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
