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

function sectionRows(count, cols) {
  return count ? Math.ceil(count / cols) : 0
}

export function sectionsHeight(sections, cols) {
  const rows = [sections.items.length, sections.cargo.length].map((n) => sectionRows(n, cols))
  const used = rows.filter(Boolean)
  if (!used.length) return 0
  const total = used.reduce((a, b) => a + b, 0)
  return total * (SLOT_PX + SLOT_GAP_PX) - SLOT_GAP_PX * used.length + SECTION_GAP_PX * (used.length - 1)
}

export function columnsForWidth(width) {
  return Math.max(1, Math.floor((width + SLOT_GAP_PX) / (SLOT_PX + SLOT_GAP_PX)))
}

// One column count for both sections, so their pockets line up. Prefers the widest grid that
// splits every section into full rows (8 × 8 for a bank) and still fits the height, else the
// grid with the fewest rows.
export function inventoryColumns(sections, maxCols, maxHeight = Infinity) {
  const counts = [sections.items.length, sections.cargo.length].filter(Boolean)
  if (!counts.length) return Math.max(1, maxCols)
  const largest = Math.max(...counts)
  const minCols = Math.min(maxCols, 4)
  for (let cols = Math.min(maxCols, largest); cols >= minCols; cols -= 1) {
    if (counts.every((n) => n % cols === 0) && sectionsHeight(sections, cols) <= maxHeight) return cols
  }
  return Math.max(1, Math.min(maxCols, largest))
}

export function compactQuantity(quantity) {
  const n = Number(quantity) || 0
  if (n < 10000) return String(n)
  if (n < 1e6) return `${Math.floor(n / 1000)}K`
  return `${Math.floor(n / 1e5) / 10}M`
}
