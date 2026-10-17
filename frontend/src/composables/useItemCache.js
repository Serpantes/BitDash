import { reactive } from 'vue'

const cache = reactive({})

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

  function normalizeMeta(raw, type, fallbackId = null) {
    if (typeof raw === 'string') return { name: raw, type }
    if (!raw || typeof raw !== 'object') return null
    return {
      name: raw.name ?? raw.itemName ?? raw.item_name ?? (fallbackId != null ? `#${fallbackId}` : null),
      iconAssetName: raw.iconAssetName ?? raw.icon_asset_name ?? null,
      tier: raw.tier ?? raw.itemTier ?? raw.item_tier ?? null,
      rarityStr: raw.rarityStr ?? raw.itemRarityStr ?? raw.item_rarity_str ?? null,
      type,
    }
  }

  function setMeta(itemType, itemId, raw) {
    const id = itemId == null ? null : Number(itemId)
    if (!Number.isFinite(id)) return
    const type = itemType === 1 ? 1 : 0
    const meta = normalizeMeta(raw, type, id)
    if (!meta) return
    if (!meta.name) meta.name = `#${id}`
    cache[cacheKey(type, id)] = meta
  }

  function catalogEntries(value) {
    if (Array.isArray(value)) {
      return value.map((item) => [item.id ?? item.itemId ?? item.item_id, item])
    }
    if (value && typeof value === 'object') {
      return Object.entries(value).map(([id, item]) => [item?.id ?? item?.itemId ?? item?.item_id ?? id, item])
    }
    return []
  }

  function seedCatalog(source) {
    const items = catalogEntries(source?.items)
    const cargos = catalogEntries(source?.cargos ?? source?.cargo)
    items.forEach(([id, item]) => setMeta(0, id, item))
    cargos.forEach(([id, cargo]) => setMeta(1, id, cargo))
  }

  function resolveName(contents) {
    return resolveItem(contents).name
  }

  function resolveItem(contents) {
    if (!contents) return { name: '?', type: 0 }
    const id = contents.itemId ?? contents.item_id
    const isCargo = contents.itemType === 1 || contents.item_type === 'cargo'
    const type = isCargo ? 1 : 0
    if (!id) return { name: '?', type }
    return get(type, id) ?? { name: `#${id}`, type }
  }

  async function prefetch(inventories) {
    if (!Array.isArray(inventories)) {
      seedCatalog(inventories)
      inventories = inventories?.inventories ?? []
    }

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
        if (!has(type, id)) requests.push({ id: Number(id), type })
      }
    }

    await fetchMissingMeta(requests)
  }

  async function prefetchItems(items) {
    if (!Array.isArray(items) || !items.length) return
    const requests = []
    for (const item of items) {
      const id = Number(item.id ?? item.itemId ?? item.item_id)
      if (!Number.isFinite(id)) continue
      const type = item.itemType === 1 || item.type === 1 ? 1 : 0
      if (!has(type, id)) requests.push({ id, type })
    }
    await fetchMissingMeta(requests)
  }

  async function fetchMissingMeta(requests) {
    const seen = new Set()
    const unique = requests.filter((r) => {
      const key = cacheKey(r.type, r.id)
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    if (unique.length === 0) return

    try {
      const res = await fetch('/api/item-names', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(unique),
      })
      if (!res.ok) return
      const data = await res.json()
      for (const [key, meta] of Object.entries(data)) {
        const type = key.startsWith('c:') ? 1 : 0
        cache[key] = normalizeMeta(meta, type) ?? { name: String(meta), type }
      }
    } catch {
    }
  }

  return { resolveName, resolveItem, prefetch, prefetchItems, seedCatalog, cache }
}
