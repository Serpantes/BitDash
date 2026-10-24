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

const emptyTierStock = () => ({ chunks: 0, packages: 0, pieces: 0, concentrates: 0, molten: 0, effortLeft: 0 })

// Sums the per-tier ore counts of the tracked inventories and, when asked, of the player's
// crafts: a craft's output counts as gathered (finished ingots excepted), and a smithing step
// still in progress adds the effort it has left.
export function trackedStock(stock, trackedIds, { crafts = false } = {}) {
  const tracked = new Set((trackedIds ?? []).map(String))
  const total = {}
  for (const inv of stock?.inventories ?? []) {
    if (!tracked.has(String(inv.id))) continue
    for (const [tier, counts] of Object.entries(inv.counts ?? {})) {
      total[tier] ??= emptyTierStock()
      for (const [kind, n] of Object.entries(counts)) total[tier][kind] = (total[tier][kind] ?? 0) + n
    }
  }
  if (crafts) {
    for (const craft of stock?.crafts ?? []) {
      const tier = String(craft.tier)
      total[tier] ??= emptyTierStock()
      if (craft.kind !== 'ingots') total[tier][craft.kind] += craft.output
      total[tier].effortLeft += craft.remainingEffort ?? 0
    }
  }
  return total
}

function chainXp(chain, setup) {
  const progress = setup.progressPerSwing
  const swingXp = Math.ceil(chain.xpPerProgress * progress * setup.xpRate)
  const xpConcentrate = progress > 0 ? (chain.concentrateEffort / progress) * swingXp : 0
  const xpForge = progress > 0 ? (chain.forgeEffort / progress) * swingXp : 0
  return {
    progress,
    swingXp,
    xpForge,
    xpPerIngot: chain.concentratesPerIngot * xpConcentrate + xpForge,
    piecesPerIngot: chain.concentratesPerIngot * chain.piecesPerConcentrate,
  }
}

// Gathered ore counts for the XP it still has to give: raw ore for the whole chain,
// concentrates and molten metal only for the steps left after them.
function stockXp(chain, xp, have) {
  const rawPieces = (have.chunks ?? 0) * chain.piecesPerChunk
    + (have.packages ?? 0) * (chain.piecesPerPackage ?? 0)
    + (have.pieces ?? 0)
  return (xp.piecesPerIngot ? (rawPieces / xp.piecesPerIngot) * xp.xpPerIngot : 0)
    + (have.concentrates ?? 0) * (xp.xpForge / chain.concentratesPerIngot)
    + (have.molten ?? 0) * xp.xpForge
    + (xp.progress > 0 ? ((have.effortLeft ?? 0) / xp.progress) * xp.swingXp : 0)
}

// How many chunks' worth of the chain some gathered ore stands for.
export function stockAsChunks(chain, setup, have) {
  const xp = chainXp(chain, setup)
  if (!xp.xpPerIngot || !chain.piecesPerChunk) return 0
  return (stockXp(chain, xp, have ?? {}) / xp.xpPerIngot) * xp.piecesPerIngot / chain.piecesPerChunk
}

export function orePlanRow(chain, { xpLeft, setup, level, market, stock }) {
  const xp = chainXp(chain, setup)
  const { progress, swingXp, xpPerIngot, piecesPerIngot } = xp
  const effortPerIngot = chain.concentratesPerIngot * chain.concentrateEffort + chain.forgeEffort
  const ingots = xpPerIngot > 0 ? Math.ceil(Math.max(0, xpLeft) / xpPerIngot) : 0
  const swings = progress > 0 ? Math.ceil((ingots * effortPerIngot) / progress) : 0
  const lockedByLevel = level < chain.levelRequired
  const lockedByTool = (setup.tool?.level ?? 0) < chain.toolLevelRequired

  const have = stock ?? {}
  const xpStillNeeded = Math.max(0, xpLeft - stockXp(chain, xp, have))
  const piecesFor = (xp) => (xpPerIngot > 0 ? Math.ceil((xp / xpPerIngot) * piecesPerIngot) : 0)
  const chunksFor = (pieces) => (chain.piecesPerChunk ? Math.ceil(pieces / chain.piecesPerChunk) : 0)
  const piecesTotal = piecesFor(Math.max(0, xpLeft))
  const piecesLeft = piecesFor(xpStillNeeded)
  const chunksTotal = chunksFor(piecesTotal)
  const chunksLeft = chunksFor(piecesLeft)
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
    // Total is what the goal takes from scratch, left is after everything gathered; have is
    // the difference, so total − have = left always holds.
    chunksTotal,
    chunks: chunksLeft,
    haveChunks: chunksTotal - chunksLeft,
    piecesTotal,
    pieces: piecesLeft,
    havePieces: piecesTotal - piecesLeft,
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

// --- Gathering speed ---------------------------------------------------------------------
// Raw ore in the tracked places, sampled on every refresh. Only growth between samples counts,
// so processing ore into concentrates or ingots never reads as negative gathering.

const GATHER_KEEP_SEC = 6 * 3600
const GATHER_MAX_SAMPLES = 3000

// Raw ore of one tier as chunks: chunks, loose pieces and packed pieces.
export function rawChunks(chain, counts) {
  if (!counts || !chain?.piecesPerChunk) return 0
  const pieces = (counts.pieces ?? 0) + (counts.packages ?? 0) * (chain.piecesPerPackage ?? 0)
  return (counts.chunks ?? 0) + pieces / chain.piecesPerChunk
}

// Adds a sample ({ t, byTier }) to the history, restarting it when the tracked set changed.
export function appendGatherSample(history, { t, signature, byTier }) {
  const keepFrom = t - GATHER_KEEP_SEC
  const samples = history?.signature === signature
    ? (history.samples ?? []).filter((s) => s.t >= keepFrom)
    : []
  const last = samples[samples.length - 1]
  if (last && t - last.t < 5) return { signature, samples }
  samples.push({ t, byTier })
  return { signature, samples: samples.slice(-GATHER_MAX_SAMPLES) }
}

// Chunks gathered per hour over the last window, or null while there is too little to tell.
export function gatherRate(history, tier, now, { windowSec = 1800, minSpanSec = 120 } = {}) {
  const key = String(tier)
  const samples = (history?.samples ?? []).filter((s) => s.t >= now - windowSec)
  if (samples.length < 2) return null
  const span = samples[samples.length - 1].t - samples[0].t
  if (span < minSpanSec) return null
  let gained = 0
  for (let i = 1; i < samples.length; i += 1) {
    const delta = (samples[i].byTier[key] ?? 0) - (samples[i - 1].byTier[key] ?? 0)
    if (delta > 0) gained += delta
  }
  return { perHour: (gained / span) * 3600, spanSec: span }
}
