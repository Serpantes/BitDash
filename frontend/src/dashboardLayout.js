export function cellKey(x, y) {
  return `${x}:${y}`
}

function validPosition(pos) {
  const x = Number(pos?.x)
  const y = Number(pos?.y)
  return Number.isInteger(x) && Number.isInteger(y) && x >= 0 && y >= 0 ? { x, y } : null
}

export function savedTilePosition(item, cols) {
  const layouts = item?.dashboardLayouts
  if (layouts && typeof layouts === 'object') {
    const keys = Object.keys(layouts).map(Number).filter((k) => Number.isInteger(k) && k > 0)
    const wider = keys.filter((k) => k >= cols).sort((a, b) => a - b)
    const narrower = keys.filter((k) => k < cols).sort((a, b) => b - a)
    for (const key of [...wider, ...narrower]) {
      const pos = validPosition(layouts[key])
      if (pos) return pos
    }
  }
  return validPosition(item?.dashboardLayout)
}

function canOccupy(occupied, cols, x, y, span) {
  if (x < 0 || y < 0 || x + span > cols) return false
  for (let i = 0; i < span; i += 1) {
    if (occupied.has(cellKey(x + i, y))) return false
  }
  return true
}

function occupy(occupied, x, y, span, id) {
  for (let i = 0; i < span; i += 1) occupied.set(cellKey(x + i, y), id)
}

function firstFreeCell(occupied, cols, span) {
  for (let y = 0; ; y += 1) {
    for (let x = 0; x + span <= cols; x += 1) {
      if (canOccupy(occupied, cols, x, y, span)) return { x, y }
    }
  }
}

export function computeTileLayout(tiles, cols) {
  const width = Math.max(1, cols)
  const list = tiles.map((tile, index) => ({ ...tile, span: Math.min(Math.max(1, tile.span), width), index }))
  const bySaved = (a, b) => a.saved.y - b.saved.y || a.saved.x - b.saved.x || a.index - b.index
  const occupied = new Map()
  const raw = {}
  const pending = []

  list.filter((tile) => tile.saved).sort(bySaved).forEach((tile) => {
    const { x, y } = tile.saved
    if (canOccupy(occupied, width, x, y, tile.span)) {
      occupy(occupied, x, y, tile.span, tile.id)
      raw[tile.id] = { x, y, span: tile.span }
    } else {
      pending.push(tile)
    }
  })
  pending.push(...list.filter((tile) => !tile.saved))
  pending.forEach((tile) => {
    const { x, y } = firstFreeCell(occupied, width, tile.span)
    occupy(occupied, x, y, tile.span, tile.id)
    raw[tile.id] = { x, y, span: tile.span }
  })

  const usedRows = [...new Set(Object.values(raw).map((pos) => pos.y))].sort((a, b) => a - b)
  const rowIndex = new Map(usedRows.map((y, i) => [y, i]))
  const positions = {}
  const compact = new Map()
  Object.entries(raw).forEach(([id, pos]) => {
    const y = rowIndex.get(pos.y)
    positions[id] = { x: pos.x, y, span: pos.span }
    occupy(compact, pos.x, y, pos.span, id)
  })
  return { positions, occupied: compact, rows: usedRows.length, cols: width }
}

export function evaluateTileDrop(layout, id, x, y) {
  const origin = layout.positions[id]
  if (!origin) return null
  const { span } = origin
  const cols = layout.cols
  const target = { x, y, span, valid: false, same: false, swap: null }
  if (x < 0 || y < 0 || x + span > cols) return target
  if (x === origin.x && y === origin.y) return { ...target, same: true }

  const others = new Map([...layout.occupied].filter(([, owner]) => owner !== id))
  const covered = new Set()
  for (let i = 0; i < span; i += 1) {
    const owner = others.get(cellKey(x + i, y))
    if (owner) covered.add(owner)
  }
  if (covered.size === 0) return { ...target, valid: true }
  if (covered.size > 1) return target

  const otherId = [...covered][0]
  const other = layout.positions[otherId]
  const fits = (tx, ty, tspan, avoid) => {
    if (tx < 0 || tx + tspan > cols) return false
    for (let i = 0; i < tspan; i += 1) {
      const owner = others.get(cellKey(tx + i, ty))
      if (owner && owner !== otherId) return false
      if (avoid && ty === avoid.y && tx + i >= avoid.x && tx + i < avoid.x + avoid.span) return false
    }
    return true
  }

  const swapX = Math.min(origin.x, cols - other.span)
  if (fits(x, y, span) && fits(swapX, origin.y, other.span, { x, y, span })) {
    return { ...target, valid: true, swap: { id: otherId, x: swapX, y: origin.y, span: other.span } }
  }
  if (origin.y === y && other.y === y) {
    const start = Math.min(origin.x, other.x)
    const movingRight = x > origin.x
    const draggedX = movingRight ? start + other.span : start
    const otherX = movingRight ? start : start + span
    if (fits(draggedX, y, span) && fits(otherX, y, other.span, { x: draggedX, y, span })) {
      return { ...target, x: draggedX, valid: true, swap: { id: otherId, x: otherX, y, span: other.span } }
    }
  }
  return target
}

export function applyTileDrop(layout, id, drop) {
  const next = {}
  Object.entries(layout.positions).forEach(([tileId, pos]) => { next[tileId] = { x: pos.x, y: pos.y } })
  next[id] = { x: drop.x, y: drop.y }
  if (drop.swap) next[drop.swap.id] = { x: drop.swap.x, y: drop.swap.y }
  return next
}
