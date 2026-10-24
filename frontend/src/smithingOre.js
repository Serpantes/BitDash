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

export function orePlanRow(chain, { xpLeft, setup, level }) {
  const progress = setup.progressPerSwing
  const swingXp = Math.ceil(chain.xpPerProgress * progress * setup.xpRate)
  const effortPerIngot = chain.concentratesPerIngot * chain.concentrateEffort + chain.forgeEffort
  const xpPerIngot = progress > 0 ? (effortPerIngot / progress) * swingXp : 0
  const ingots = xpPerIngot > 0 ? Math.ceil(Math.max(0, xpLeft) / xpPerIngot) : 0
  const piecesPerIngot = chain.concentratesPerIngot * chain.piecesPerConcentrate
  const swings = progress > 0 ? Math.ceil((ingots * effortPerIngot) / progress) : 0
  const lockedByLevel = level < chain.levelRequired
  const lockedByTool = (setup.tool?.level ?? 0) < chain.toolLevelRequired
  return {
    ...chain,
    swingXp,
    xpPerIngot,
    xpPerPiece: piecesPerIngot ? xpPerIngot / piecesPerIngot : 0,
    piecesPerIngot,
    ingots,
    pieces: ingots * piecesPerIngot,
    chunks: chain.piecesPerChunk ? Math.ceil((ingots * piecesPerIngot) / chain.piecesPerChunk) : null,
    flux: ingots * chain.fluxPerIngot,
    swings,
    seconds: swings * (chain.swingTime / (setup.speed || 1)),
    locked: lockedByLevel || lockedByTool,
    lockedByLevel,
    lockedByTool,
  }
}

export function orePlan(chains, { xpLeft, setup, level }) {
  const rows = chains.filter(Boolean).map((chain) => orePlanRow(chain, { xpLeft, setup, level }))
  const open = rows.filter((row) => !row.locked && row.ingots > 0)
  const best = open.reduce((a, b) => (!a || b.pieces < a.pieces ? b : a), null)
  return rows
    .map((row) => ({ ...row, best: row === best }))
    .sort((a, b) => b.tier - a.tier)
}
