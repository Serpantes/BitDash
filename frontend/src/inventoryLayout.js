import { shallowRef } from 'vue'

// Inventory pockets as the game lays them out: item pockets first, then cargo pockets, each
// in pocket order. BitJita lists only filled pockets; the backend adds `slot` (the pocket index
// within its section), exact from the game's database replica when it has the storage.

export const SLOT_GAP_PX = 3
// The gap between the item and cargo pockets, and between panels drawn side by side.
export const SECTION_GAP_PX = 9
export const PANEL_GAP_PX = 14
// Pockets are taller than wide, as in game (52 × 64).
export const SLOT_RATIO = 64 / 52
// Below this width a tile is too cramped and the storage takes a wide one.
export const MIN_SLOT_PX = 26
const MAX_SLOT_PX = 60

const CAPACITY_STORAGE_KEY = 'bitdash.storageCapacity'
const CAPACITY_TTL_MS = 24 * 60 * 60 * 1000

function readStoredCapacity() {
  try {
    const stored = JSON.parse(localStorage.getItem(CAPACITY_STORAGE_KEY) ?? 'null')
    return stored?.storages ? stored : null
  } catch {
    return null
  }
}

// Item and cargo pocket counts by building or deployable name, from /api/storage-capacity.
export const storageCapacities = shallowRef(readStoredCapacity()?.storages ?? null)
let capacityPending = null

export function ensureStorageCapacities() {
  const stored = readStoredCapacity()
  if (capacityPending || (stored && Date.now() - stored.savedAt < CAPACITY_TTL_MS)) return capacityPending
  capacityPending = fetch('/api/storage-capacity')
    .then((res) => (res.ok ? res.json() : null))
    .then((payload) => {
      if (!payload?.storages) return
      storageCapacities.value = payload.storages
      try {
        localStorage.setItem(CAPACITY_STORAGE_KEY, JSON.stringify({ savedAt: Date.now(), storages: payload.storages }))
      } catch { /* storage full or blocked: keep it in memory */ }
    })
    .catch(() => {})
    .finally(() => { capacityPending = null })
  return capacityPending
}

export function hasContents(pocket) {
  const contents = pocket?.contents
  return Boolean(contents && (contents.itemId != null || contents.item_id != null))
}

export function isCargoPocket(pocket) {
  const contents = pocket?.contents
  return contents?.itemType === 1 || contents?.item_type === 'cargo'
}

// "Serpantes's Personal Cache (III)" is the deployable "Personal Cache (III)".
function deployableName(name) {
  return String(name ?? '').replace(/^.*?'s\s+/, '')
}

export function storageCapacity(meta) {
  const table = storageCapacities.value ?? {}
  const known = [meta?.buildingName, deployableName(meta?.inventoryName), meta?.inventoryName]
    .map((name) => table[name])
    .find(Boolean)
  // The backend adds exact counts when the game's database replica holds the storage.
  if (Number.isInteger(meta?.itemPockets) && Number.isInteger(meta?.cargoPockets)) {
    return { items: meta.itemPockets, cargo: meta.cargoPockets }
  }
  // Player inventories carry the count of item pockets (the index where cargo pockets start).
  const cargoIndex = Number(meta?.cargoIndex)
  return {
    items: Number.isFinite(cargoIndex) && cargoIndex > 0 ? cargoIndex : (known?.[0] ?? 0),
    cargo: known?.[1] ?? 0,
  }
}

// One section's pockets in place: an array of pockets and nulls for empty ones. Pockets go to
// their remembered slot when every slot fits; otherwise they are packed in listing order.
export function placeSection(pockets, capacity) {
  const slots = pockets.map((p) => Number(p.slot))
  const valid = slots.every((s) => Number.isInteger(s) && s >= 0 && (!capacity || s < capacity))
    && new Set(slots).size === slots.length
  const length = valid
    ? Math.max(capacity, slots.length ? Math.max(...slots) + 1 : 0)
    : Math.max(capacity, pockets.length)
  const out = Array.from({ length }, () => null)
  pockets.forEach((pocket, i) => { out[valid ? slots[i] : i] = pocket })
  return out
}

export function inventorySections(pockets, meta) {
  const filled = (pockets ?? []).filter(hasContents)
  const capacity = storageCapacity(meta)
  return {
    items: placeSection(filled.filter((p) => !isCargoPocket(p)), capacity.items),
    cargo: placeSection(filled.filter(isCargoPocket), capacity.cargo),
  }
}

// The game shows storages ten item pockets wide; a cargo pocket takes two, so five per row.
export const GAME_COLS = 10

function cargoPerRow(cols) {
  return Math.max(1, Math.floor(cols / 2))
}

// Row widths that keep every game row whole: half a row, a row, or two rows side by side.
// Half rows are for item-only storages; cargo would break its five-per-row runs.
function rowWidthChoices(sections) {
  return sections.cargo.length ? [GAME_COLS, GAME_COLS * 2] : [GAME_COLS, GAME_COLS / 2, GAME_COLS * 2]
}

function perRow(kind, cols) {
  return kind === 'cargo' ? cargoPerRow(cols) : cols
}

// How the pockets are drawn: panels side by side, each holding whole rows of one or both
// sections ("parts", with the pocket index each starts at). Stacked, the cargo goes under the
// items in one panel; split, items and cargo get a panel each, and a lone section is cut in two
// halves of whole rows, so every row still reads as in game.
export function inventoryPanels(sections, split, cols = GAME_COLS) {
  const parts = [
    { kind: 'item', pockets: sections.items, offset: 0 },
    { kind: 'cargo', pockets: sections.cargo, offset: 0 },
  ].filter((part) => part.pockets.length)
  if (!split) return parts.length ? [{ parts }] : []
  if (parts.length > 1) return parts.map((part) => ({ parts: [part] }))
  if (!parts.length) return []
  const [part] = parts
  const rowSize = perRow(part.kind, cols)
  const rows = Math.ceil(part.pockets.length / rowSize)
  if (rows < 2) return [{ parts }]
  const cut = Math.ceil(rows / 2) * rowSize
  return [
    { parts: [{ ...part, pockets: part.pockets.slice(0, cut) }] },
    { parts: [{ ...part, pockets: part.pockets.slice(cut), offset: cut }] },
  ]
}

// The widest pocket that lets every panel fit the given box.
export function slotWidth(panels, width, height, cols = GAME_COLS) {
  if (!panels.length) return MAX_SLOT_PX
  const panelWidth = (width - PANEL_GAP_PX * (panels.length - 1)) / panels.length
  // Each pocket keeps half a gap on either side.
  const byWidth = (panelWidth - 1) / cols - SLOT_GAP_PX
  const byHeight = Math.min(...panels.map(({ parts }) => {
    const rows = parts.reduce((sum, part) => sum + Math.ceil(part.pockets.length / perRow(part.kind, cols)), 0)
    const usable = height - SECTION_GAP_PX * (parts.length - 1) + SLOT_GAP_PX * parts.length
    return (usable / rows - SLOT_GAP_PX) / SLOT_RATIO
  }))
  return Math.max(0, Math.min(MAX_SLOT_PX, byWidth, byHeight))
}

// The row width (5, 10 or 20 pockets) with the largest pockets for the box, ten unless another
// is clearly better, and the panels and pocket width that go with it.
export function inventoryLayout(sections, split, width, height) {
  let best = null
  for (const cols of rowWidthChoices(sections)) {
    const panels = inventoryPanels(sections, split, cols)
    const slot = slotWidth(panels, width, height, cols)
    if (!best || slot > best.slot * 1.05) best = { cols, panels, slot }
  }
  return best
}

export function compactQuantity(quantity) {
  const n = Number(quantity) || 0
  if (n < 10000) return String(n)
  if (n < 1e6) return `${Math.floor(n / 1000)}K`
  return `${Math.floor(n / 1e5) / 10}M`
}

// Where every filled pocket of a section goes when the stack at `from` is dropped on `to`.
// Pockets keep their in-game order (BitJita lists them that way), so the stacks in the way are
// pushed along; null when they would be pushed off the grid.
export function moveInSection(placed, from, to) {
  const filled = placed.map((pocket, i) => (pocket ? i : -1)).filter((i) => i >= 0)
  const k = filled.indexOf(from)
  if (k < 0 || from === to || to < 0 || to >= placed.length) return null
  const slots = [...filled]
  slots[k] = to
  for (let i = k + 1; i < slots.length; i += 1) slots[i] = Math.max(slots[i], slots[i - 1] + 1)
  for (let i = k - 1; i >= 0; i -= 1) slots[i] = Math.min(slots[i], slots[i + 1] - 1)
  if (slots[0] < 0 || slots[slots.length - 1] >= placed.length) return null
  return filled.map((index, i) => ({ pocket: placed[index], slot: slots[i] }))
}

// Shows a hand-arranged section at once and has the backend keep it from now on.
export async function saveArrangement(inventoryId, kind, moves) {
  moves.forEach(({ pocket, slot }) => { pocket.slot = slot })
  const placements = moves.map(({ pocket, slot }) => ({
    slot,
    itemId: String(pocket.contents.itemId ?? pocket.contents.item_id),
    quantity: String(pocket.contents.quantity ?? ''),
  }))
  const res = await fetch('/api/inventory-slots', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ inventoryId: String(inventoryId), [kind === 'cargo' ? 'cargo' : 'items']: placements }),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}
