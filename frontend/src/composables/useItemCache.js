// In-memory item name cache backed by the backend (/api/item-names).
// Names are persisted on the server side (data/item-names.json).
// The in-memory map avoids re-requesting names already known in this session.

import { reactive } from 'vue'

// Reactive so templates update after prefetch fills the cache (plain objects would not re-render).
const cache = reactive({}) // key -> name, e.g. "i:1000" -> "Stone"

/**
 * Build the cache key used by both this module and the backend store.
 * itemType 1 = cargo → "c:{id}", otherwise → "i:{id}"
 */
function cacheKey(itemType, itemId) {
  return itemType === 1 ? `c:${itemId}` : `i:${itemId}`
}

export function useItemCache() {
  function has(itemType, itemId) {
    return cacheKey(itemType, itemId) in cache
  }

  function get(itemType, itemId) {
    return cache[cacheKey(itemType, itemId)] ?? null
  }

  /**
   * Resolve display name for a pocket contents object.
   * Handles both formats:
   *   - regular inventory: { itemId, itemType: 0|1 }
   *   - housing inventory: { item_id, item_type: "item"|"cargo" }
   * Returns cached name or "#id" fallback.
   */
  function resolveName(contents) {
    if (!contents) return '?'
    const id = contents.itemId ?? contents.item_id
    if (!id) return '?'
    const isCargo = contents.itemType === 1 || contents.item_type === 'cargo'
    return get(isCargo ? 1 : 0, id) ?? `#${id}`
  }

  /**
   * Collect all unknown item/cargo IDs from inventories, request their names
   * from the backend in a single POST /api/item-names call, and store results.
   * Handles both regular ({ pockets }) and housing ({ inventory }) formats.
   */
  async function prefetch(inventories) {
    const requests = []

    for (const inv of inventories) {
      const pockets = inv.pockets ?? inv.inventory ?? []
      for (const p of pockets) {
        const c = p.contents
        if (!c) continue
        const id = c.itemId ?? c.item_id
        if (!id) continue
        const isCargo = c.itemType === 1 || c.item_type === 'cargo'
        const type = isCargo ? 1 : 0
        if (!has(type, id)) {
          requests.push({ id: Number(id), type })
        }
      }
    }

    if (requests.length === 0) return

    // Deduplicate
    const seen = new Set()
    const unique = requests.filter((r) => {
      const k = cacheKey(r.type, r.id)
      if (seen.has(k)) return false
      seen.add(k)
      return true
    })

    try {
      const res = await fetch('/api/item-names', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(unique),
      })
      if (!res.ok) return
      const data = await res.json()
      for (const [key, name] of Object.entries(data)) {
        cache[key] = name
      }
    } catch {
      // Non-fatal; UI will show "#id" fallbacks
    }
  }

  return { resolveName, prefetch, cache }
}

