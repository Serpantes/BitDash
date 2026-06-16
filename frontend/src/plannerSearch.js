export function plannerSearchDedupeKey(entry) {
  const tier = Number(entry.tier) || 0
  const tag = entry.tag
  if (tag && tag !== 'Recipe') return `${entry.itemType}:${tier}:${tag}`
  return `${entry.itemType}:${tier}:${entry.name ?? ''}`
}

export function normalizePlannerSearchEntry(raw, itemType) {
  const id = raw?.id ?? raw?.itemId ?? raw?.item_id
  const name = raw?.name ?? raw?.itemName ?? 'Предмет'
  return {
    id: String(id),
    itemType,
    tag: raw?.tag ?? null,
    tier: raw?.tier ?? raw?.itemTier ?? null,
  }
}

function shouldSkipPlannerSearchEntry(entry, name) {
  if (!entry.id) return true
  if (/ output$/i.test(name)) return true
  if (/high quality/i.test(name)) return true
  if (entry.tag === 'Recipe') return true
  if (Number(entry.tier) < 0) return true
  return false
}

/** One entry per tier + item type; rarity variants collapse to a single row. */
export function dedupePlannerSearchResults(entries, itemMeta) {
  const filtered = entries.filter((e) => !shouldSkipPlannerSearchEntry(e, itemMeta(e).name))
  const byKey = new Map()
  for (const entry of filtered) {
    const key = plannerSearchDedupeKey({ ...entry, name: itemMeta(entry).name })
    if (!byKey.has(key)) byKey.set(key, entry)
  }
  return [...byKey.values()]
    .sort((a, b) => {
      const tierDiff = (Number(a.tier) || 0) - (Number(b.tier) || 0)
      if (tierDiff !== 0) return tierDiff
      return itemMeta(a).name.localeCompare(itemMeta(b).name, 'ru')
    })
    .slice(0, 16)
}
