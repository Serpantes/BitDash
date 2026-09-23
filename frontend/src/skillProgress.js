/**
 * Skill leveling math on top of BitJita `/static/experience/levels.json`
 * ([{ level, xp }] — cumulative XP needed to reach each level, 1..120).
 * Each level costs ~11.6% more XP than the previous one, so level progress
 * and XP progress towards 100 diverge a lot (level 90 ≈ 33% of the XP).
 */

export const SKILL_TARGET_LEVEL = 100

export const SKILL_CATEGORIES = [
  { id: 'profession', label: 'Профессии' },
  { id: 'adventure', label: 'Приключения' },
]

const SKILL_GLYPHS = {
  2: '🌲',
  3: '🪚',
  4: '🧱',
  5: '⛏️',
  6: '⚒️',
  7: '📜',
  8: '🧤',
  9: '🏹',
  10: '🪡',
  11: '🌾',
  12: '🎣',
  13: '🍲',
  14: '🍄',
  15: '🏗️',
  17: '🐾',
  18: '⚔️',
  19: '💰',
  21: '⛵',
}

const PATH_TICK_LEVELS = [80, 90, 95]

export function skillGlyph(skillId) {
  return SKILL_GLYPHS[Number(skillId)] ?? '★'
}

/** levels.json rows → cumulative XP thresholds, index = level - 1. */
export function parseLevelTable(rows) {
  if (!Array.isArray(rows)) return null
  const thresholds = rows
    .map((row) => ({ level: Number(row?.level), xp: Number(row?.xp) }))
    .filter((row) => Number.isInteger(row.level) && row.level >= 1 && Number.isFinite(row.xp))
    .sort((a, b) => a.level - b.level)
    .map((row) => row.xp)
  return thresholds.length > 2 ? thresholds : null
}

export function levelForXp(table, xp) {
  let level = 1
  for (let i = 1; i < table.length; i += 1) {
    if (xp < table[i]) break
    level = i + 1
  }
  return level
}

export function skillProgress(table, rawXp, target = SKILL_TARGET_LEVEL) {
  if (!table?.length) return null
  const xp = Math.max(0, Number(rawXp) || 0)
  const maxLevel = table.length
  const level = levelForXp(table, xp)
  const isMax = level >= maxLevel
  const levelStartXp = table[level - 1]
  const nextLevelXp = isMax ? levelStartXp : table[level]
  const levelSpan = nextLevelXp - levelStartXp
  const levelPct = isMax || levelSpan <= 0 ? 100 : ((xp - levelStartXp) / levelSpan) * 100
  const targetLevel = Math.min(target, maxLevel)
  const targetXp = table[targetLevel - 1]
  return {
    xp,
    level,
    maxLevel,
    isMax,
    levelStartXp,
    nextLevelXp,
    levelXp: xp - levelStartXp,
    levelSpan,
    levelPct,
    toNext: isMax ? 0 : nextLevelXp - xp,
    targetLevel,
    targetXp,
    targetPct: targetXp > 0 ? Math.min(100, (xp / targetXp) * 100) : 100,
    toTarget: Math.max(0, targetXp - xp),
    reachedTarget: level >= targetLevel,
    // Fractional level, for placing the player on the curve.
    position: isMax ? level : level + levelPct / 100,
  }
}

/** Level marks on the XP-to-target bar (80 / 90 / 95 sit at 11% / 33% / 58%). */
export function targetPathTicks(table, targetLevel) {
  const targetXp = table[targetLevel - 1]
  return PATH_TICK_LEVELS
    .filter((level) => level < targetLevel)
    .map((level) => ({ level, pct: (table[level - 1] / targetXp) * 100 }))
}

/** Shape of the curve in words: per-level growth and how few top levels hold half the XP. */
export function curveFacts(table, targetLevel) {
  const targetXp = table[targetLevel - 1]
  const lastStep = targetXp - table[targetLevel - 2]
  const prevStep = table[targetLevel - 2] - table[targetLevel - 3]
  let halfFrom = 1
  for (let level = 1; level <= targetLevel; level += 1) {
    if (table[level - 1] <= targetXp / 2) halfFrom = level
  }
  return {
    growthPct: prevStep > 0 ? (lastStep / prevStep - 1) * 100 : 0,
    halfFromLevel: halfFrom,
    halfLevels: targetLevel - halfFrom,
  }
}

function niceStep(max, maxTicks) {
  const raw = max / maxTicks
  const magnitude = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= raw)
  return step ?? 10 * magnitude
}

/**
 * Geometry for the cumulative-XP curve (x = level, y = total XP), in viewBox units.
 * Past the target the domain extends to the level cap.
 */
export function skillCurveGeometry(table, progress, box) {
  const { width, height, left, right, top, bottom } = box
  const domainMax = progress.reachedTarget ? table.length : progress.targetLevel
  const yMax = table[domainMax - 1]
  const plotW = width - left - right
  const plotH = height - top - bottom
  const x = (level) => left + ((level - 1) / (domainMax - 1)) * plotW
  const y = (xp) => top + plotH - (xp / yMax) * plotH
  const xpAt = (position) => {
    const base = Math.floor(position)
    if (base >= domainMax) return table[domainMax - 1]
    return table[base - 1] + (position - base) * (table[base] - table[base - 1])
  }

  const levels = Array.from({ length: domainMax }, (_, i) => i + 1)
  const toPath = (points) => points.map(([px, py], i) => `${i ? 'L' : 'M'}${px.toFixed(1)},${py.toFixed(1)}`).join('')
  const linePath = toPath(levels.map((level) => [x(level), y(table[level - 1])]))

  const position = Math.min(progress.position, domainMax)
  const doneLevels = levels.filter((level) => level < position)
  const donePoints = [...doneLevels.map((level) => [x(level), y(table[level - 1])]), [x(position), y(xpAt(position))]]
  const baseline = y(0)
  const doneLinePath = toPath(donePoints)
  const doneAreaPath = `${toPath([[x(1), baseline], ...donePoints, [x(position), baseline]])}Z`

  const yStep = niceStep(yMax, 5)
  const yTicks = []
  for (let v = 0; v <= yMax; v += yStep) yTicks.push({ value: v, y: y(v) })
  const xTicks = [1, ...levels.filter((level) => level % 20 === 0)].map((level) => ({ level, x: x(level) }))

  return {
    domainMax,
    plot: { left, top, right: width - right, bottom: height - bottom, width: plotW, height: plotH },
    linePath,
    doneLinePath,
    doneAreaPath,
    yTicks,
    xTicks,
    dot: { x: x(position), y: y(xpAt(position)) },
    target: { x: x(progress.targetLevel), y: y(progress.targetXp) },
    x,
    y,
    levelAtX(px) {
      const level = Math.round(1 + ((px - left) / plotW) * (domainMax - 1))
      return Math.min(domainMax, Math.max(1, level))
    },
  }
}

export function formatXpCompact(value) {
  const n = Number(value) || 0
  const abs = Math.abs(n)
  const trim = (text) => text.replace(/\.0+$/, '')
  if (abs >= 1e9) return `${trim((n / 1e9).toFixed(2))}B`
  if (abs >= 1e6) return `${trim((n / 1e6).toFixed(1))}M`
  if (abs >= 1e4) return `${trim((n / 1e3).toFixed(1))}K`
  return Math.round(n).toLocaleString('ru-RU')
}

/** Three significant digits (299K, 47.7K, 1.23M) — for tight tiles. */
export function formatXpShort(value) {
  const n = Number(value) || 0
  const abs = Math.abs(n)
  const [div, suffix] = abs >= 1e9 ? [1e9, 'B'] : abs >= 1e6 ? [1e6, 'M'] : abs >= 1e3 ? [1e3, 'K'] : [1, '']
  const scaled = n / div
  const digits = Math.abs(scaled) >= 100 ? 0 : Math.abs(scaled) >= 10 ? 1 : 2
  return suffix ? `${Number(scaled.toFixed(digits))}${suffix}` : String(Math.round(n))
}

export function formatSkillPct(pct) {
  if (pct == null || !Number.isFinite(pct)) return '—'
  if (pct > 0 && pct < 1) return `${pct.toFixed(2)}%`
  if (pct < 10) return `${pct.toFixed(1)}%`
  if (pct >= 99.95 && pct < 100) return '99.9%'
  return `${pct.toFixed(1)}%`
}

export function formatTopPct(pct) {
  if (pct == null || !Number.isFinite(pct)) return '—'
  if (pct < 1) return `${pct.toFixed(2)}%`
  if (pct < 10) return `${pct.toFixed(1)}%`
  return `${Math.round(pct)}%`
}
