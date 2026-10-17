import { fetchBitjita } from './api'

export const ACTIVE_DAYS = 30

const PAGE_SIZE = 100
const SEARCH_PAGES_MAX = 5
const TOTAL_TTL_MS = 30 * 60 * 1000
const totalCache = new Map()

function skillXpOf(row, skillId) {
  return Number(row?.skillXP?.[String(skillId)]) || 0
}

function leaderboardPage(skillId, page, pageSize, activeDays, extra = {}, fresh = false) {
  const params = { sortBy: String(skillId), page: String(page), pageSize: String(pageSize), ...extra }
  if (activeDays) params.activeDays = String(activeDays)
  return fetchBitjita('/leaderboard/skills', params, { fresh })
}

export async function skillPlace(skillId, place, activeDays = null, { fresh = false } = {}) {
  const res = await leaderboardPage(skillId, place, 1, activeDays, {}, fresh)
  const row = res?.players?.[0]
  return row ? { rank: place, xp: skillXpOf(row, skillId), username: row.username ?? '' } : null
}

export async function activeSkillRank(skillId, player, activeDays = ACTIVE_DAYS, { fresh = false } = {}) {
  if (!player?.username) return null
  for (let page = 1; page <= SEARCH_PAGES_MAX; page += 1) {
    const res = await leaderboardPage(skillId, page, PAGE_SIZE, activeDays, { search: player.username }, fresh)
    const row = (res?.players ?? []).find((r) => String(r.entityId) === String(player.entityId))
    if (row) return skillXpOf(row, skillId) > 0 ? Number(row.rank) || null : null
    if (!res?.pagination?.hasNext) return null
  }
  return null
}

export function activeSkillPlayers(skillId, activeDays = ACTIVE_DAYS, { fresh = false } = {}) {
  const key = `${skillId}:${activeDays}`
  const cached = totalCache.get(key)
  if (!fresh && cached?.value != null && Date.now() - cached.at < TOTAL_TTL_MS) return Promise.resolve(cached.value)
  if (cached?.promise && (!fresh || cached.fresh)) return cached.promise
  const promise = countPlayersWithXp(skillId, activeDays, fresh)
    .then((value) => {
      totalCache.set(key, { at: Date.now(), value })
      return value
    })
    .catch((error) => {
      if (cached?.value != null) {
        totalCache.set(key, { at: cached.at, value: cached.value })
        return cached.value
      }
      totalCache.delete(key)
      throw error
    })
  totalCache.set(key, { ...cached, promise, fresh })
  return promise
}

async function countPlayersWithXp(skillId, activeDays, fresh) {
  const load = async (page) => {
    const res = await leaderboardPage(skillId, page, PAGE_SIZE, activeDays, {}, fresh)
    return { rows: res?.players ?? [], pages: Number(res?.pagination?.totalPages) || 1 }
  }
  const first = await load(1)
  if (!first.rows.length || skillXpOf(first.rows[0], skillId) <= 0) return 0
  let lo = 1
  let hi = first.pages
  let loRows = first.rows
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2)
    const { rows } = await load(mid)
    if (rows.length && skillXpOf(rows[0], skillId) > 0) {
      lo = mid
      loRows = rows
    } else {
      hi = mid - 1
    }
  }
  return (lo - 1) * PAGE_SIZE + loRows.filter((row) => skillXpOf(row, skillId) > 0).length
}
