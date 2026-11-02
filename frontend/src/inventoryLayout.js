import { shallowRef } from 'vue'

// Inventory pockets as the game lays them out: item pockets first, then cargo pockets, each
// in pocket order. BitJita lists only filled pockets; the backend adds `slot` (the pocket index
// within its section) and remembers it between listings, so stacks keep their place.

export const SLOT_PX = 32
export const SLOT_GAP_PX = 3
// The gap between the item and cargo pockets.
export const SECTION_GAP_PX = 9

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

function sectionHeights(sections, cols) {
  const rows = [
    sections.items.length ? Math.ceil(sections.items.length / cols) : 0,
    sections.cargo.length ? Math.ceil(sections.cargo.length / cargoPerRow(cols)) : 0,
  ]
  return rows.filter(Boolean).map((n) => n * (SLOT_PX + SLOT_GAP_PX) - SLOT_GAP_PX)
}

// Height of the pockets with the item and cargo sections one above the other, or side by side.
export function sectionsHeight(sections, cols, sideBySide = false) {
  const heights = sectionHeights(sections, cols)
  if (!heights.length) return 0
  if (sideBySide) return Math.max(...heights)
  return heights.reduce((a, b) => a + b, 0) + SECTION_GAP_PX * (heights.length - 1)
}

export function columnsForWidth(width) {
  return Math.max(1, Math.floor((width + SLOT_GAP_PX) / (SLOT_PX + SLOT_GAP_PX)))
}

// As wide as in game, unless the screen is narrower; cargo needs an even count.
export function inventoryColumns(maxCols = GAME_COLS) {
  const cols = Math.max(2, Math.min(GAME_COLS, maxCols))
  return cols - (cols % 2)
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
