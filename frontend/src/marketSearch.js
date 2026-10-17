import { t } from './i18n'

export const RESULT_LIMIT = 60
const MIN_SUBSTRING_QUERY = 2
const TIER_TOKEN = /^[tт](\d{1,2})$/

const RARITIES = ['uncommon', 'rare', 'epic', 'legendary', 'mythic']

export function prepareGoods(goods) {
  return (goods ?? []).map((good) => ({
    ...good,
    id: String(good.id),
    itemType: Number(good.itemType) === 1 ? 1 : 0,
    key: `${Number(good.itemType) === 1 ? 1 : 0}:${good.id}`,
    nameLower: String(good.name ?? '').toLowerCase(),
    hasSell: Boolean(good.hasSell),
    hasBuy: Boolean(good.hasBuy),
  }))
}

export function parseQuery(query) {
  const words = []
  let tier = null
  for (const token of String(query ?? '').toLowerCase().split(/\s+/)) {
    if (!token) continue
    const tierMatch = token.match(TIER_TOKEN)
    if (tierMatch && Number(tierMatch[1]) >= 1 && Number(tierMatch[1]) <= 10) tier = Number(tierMatch[1])
    else words.push(token)
  }
  return { words, tier, text: words.join(' ') }
}

const WORD_EDGE = /[^a-z0-9]/

function wordHit(nameLower, word) {
  let best = null
  for (let i = nameLower.indexOf(word); i !== -1; i = nameLower.indexOf(word, i + 1)) {
    const end = i + word.length
    const atStart = i === 0 || WORD_EDGE.test(nameLower[i - 1])
    const atEnd = end === nameLower.length || WORD_EDGE.test(nameLower[end])
    const quality = atStart ? (atEnd ? 2 : 1) : 0
    if (!best || quality > best.quality) best = { index: i, quality }
    if (quality === 2) break
  }
  return best
}

function matchRank(nameLower, { words, text }) {
  if (nameLower === text) return 0
  let weakest = 2
  for (const word of words) weakest = Math.min(weakest, wordHit(nameLower, word)?.quality ?? 0)
  return 3 - weakest
}

function tierOrder(tier) {
  return tier != null && tier > 0 ? tier : 99
}

function compareHits(a, b) {
  return Number(b.traded) - Number(a.traded)
    || a.rank - b.rank
    || tierOrder(a.good.tier) - tierOrder(b.good.tier)
    || Number(b.prefix) - Number(a.prefix)
    || a.good.name.length - b.good.name.length
    || a.good.name.localeCompare(b.good.name)
}

export function searchGoods(goods, query, limit = RESULT_LIMIT) {
  const parsed = parseQuery(query)
  if (!goods || (!parsed.words.length && parsed.tier == null)) return null
  const wordStartsOnly = parsed.text.length < MIN_SUBSTRING_QUERY
  const hits = []
  for (const good of goods) {
    if (parsed.tier != null && good.tier !== parsed.tier) continue
    if (!parsed.words.every((word) => good.nameLower.includes(word))) continue
    const rank = parsed.words.length ? matchRank(good.nameLower, parsed) : 1
    if (wordStartsOnly && rank > 2) continue
    hits.push({
      good,
      rank,
      prefix: parsed.words.length > 0 && good.nameLower.startsWith(parsed.text),
      traded: good.hasSell || good.hasBuy,
    })
  }
  hits.sort(parsed.words.length ? compareHits : (a, b) => Number(b.traded) - Number(a.traded) || a.good.name.localeCompare(b.good.name))
  const shown = hits.slice(0, limit)
  return {
    traded: shown.filter((hit) => hit.traded).map((hit) => hit.good),
    untraded: shown.filter((hit) => !hit.traded).map((hit) => hit.good),
    total: hits.length,
  }
}

export function highlightParts(name, query) {
  const text = String(name ?? '')
  const nameLower = text.toLowerCase()
  const ranges = []
  for (const word of parseQuery(query).words) {
    const hit = wordHit(nameLower, word)
    if (hit) ranges.push([hit.index, hit.index + word.length])
  }
  ranges.sort((a, b) => a[0] - b[0])
  const parts = []
  let pos = 0
  for (const [start, end] of ranges) {
    const from = Math.max(start, pos)
    if (from >= end) continue
    if (from > pos) parts.push({ text: text.slice(pos, from), hit: false })
    parts.push({ text: text.slice(from, end), hit: true })
    pos = end
  }
  if (pos < text.length) parts.push({ text: text.slice(pos), hit: false })
  return parts
}

export function rarityLabel(rarityStr) {
  const key = String(rarityStr ?? '').toLowerCase()
  return RARITIES.includes(key) ? t(`rarity.${key}`) : ''
}

export function goodMeta(good) {
  const parts = []
  if (good?.tier > 0) parts.push(`T${good.tier}`)
  if (good?.tag) parts.push(good.tag)
  const rarity = rarityLabel(good?.rarityStr)
  if (rarity) parts.push(rarity)
  if (good?.itemType === 1) parts.push(t('goods.cargo'))
  return parts.join(' · ')
}

export function queryHasCyrillic(query) {
  return /[а-яё]/i.test(parseQuery(query).text)
}
