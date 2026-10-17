import { formatNumber, intlLocale, t } from './i18n'

export const ACTIVITY_PERIODS = [
  { id: '24h', get label() { return t('units.hours', { n: 24 }) }, bucket: 3600, count: 24 },
  { id: '7d', get label() { return t('units.days', { n: 7 }) }, bucket: 86400, count: 7 },
  { id: '30d', get label() { return t('units.days', { n: 30 }) }, bucket: 86400, count: 30 },
]

const ACTIVE_GAP_SEC = 5 * 60
const RESUME_SEC = 60
const MIN_WINDOW_SEC = 60
const FORECAST_WINDOW_SEC = 30 * 86400
const MIN_FORECAST_SEC = 6 * 3600
const MAX_FORECAST_SEC = 100 * 365 * 86400
const LIVE_POINT_GAP_SEC = 15

export function splitHistorySamples(samples) {
  const bySkill = {}
  ;(samples ?? []).forEach((sample) => {
    Object.entries(sample.s ?? {}).forEach(([skill, xp]) => {
      if (!bySkill[skill]) bySkill[skill] = []
      bySkill[skill].push([Number(sample.t), Number(xp)])
    })
  })
  return bySkill
}

export function mergeHistoryPoints(prev, incoming) {
  const out = prev ? prev.slice() : []
  incoming.forEach(([t, xp]) => {
    const last = out[out.length - 1]
    if (last && t <= last[0]) return
    if (last && xp < last[1]) return
    out.push([t, xp])
  })
  return out
}

// Live socket gains arrive every few seconds; keep one point per gap so a long session
// does not grow the series by thousands of points.
export function appendLivePoint(points, t, xp, gapSec = LIVE_POINT_GAP_SEC) {
  const last = points?.[points.length - 1]
  if (last && (t <= last[0] || xp <= last[1])) return points
  const out = points ? points.slice() : []
  const prev = out[out.length - 2]
  if (last?.[2] && prev && t - prev[0] < gapSec) out[out.length - 1] = [t, xp, true]
  else out.push([t, xp, true])
  return out
}

export function xpAt(points, t) {
  let lo = 0
  let hi = points.length - 1
  let found = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (points[mid][0] <= t) {
      found = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return found < 0 ? null : points[found][1]
}

export function windowPace(points, now, sec) {
  if (!points?.length) return null
  const firstT = points[0][0]
  const start = Math.max(now - sec, firstT)
  const covered = now - start
  if (covered < MIN_WINDOW_SEC) return null
  const gained = xpAt(points, now) - (xpAt(points, start) ?? points[0][1])
  return { gained, perHour: (gained / covered) * 3600, covered, partial: covered < sec - MIN_WINDOW_SEC }
}

export function playSessions(points, from = -Infinity) {
  const sessions = []
  let current = null
  for (let i = 1; i < (points?.length ?? 0); i += 1) {
    const [t, xp] = points[i]
    const [prevT, prevXp] = points[i - 1]
    if (t <= from || xp <= prevXp) continue
    const dt = t - prevT
    const resumed = dt > ACTIVE_GAP_SEC || (current !== null && t - current.end > ACTIVE_GAP_SEC)
    if (!current || resumed) {
      current = { start: resumed ? t - RESUME_SEC : prevT, end: t, activeSec: 0, gained: 0 }
      sessions.push(current)
    }
    current.end = t
    current.activeSec += resumed ? RESUME_SEC : dt
    current.gained += xp - prevXp
  }
  return sessions
}

export function activityPeriod(id) {
  if (id === '24h' || id === '15m' || id === '1h') return ACTIVITY_PERIODS[0]
  if (id === '30d') return ACTIVITY_PERIODS[2]
  return ACTIVITY_PERIODS[1]
}

export function localBucketStart(t, size) {
  const offset = new Date().getTimezoneOffset() * 60
  return Math.floor((t - offset) / size) * size + offset
}

function forEachGain(points, visit) {
  for (let i = 1; i < (points?.length ?? 0); i += 1) {
    const [t, xp] = points[i]
    const [prevT, prevXp] = points[i - 1]
    if (xp <= prevXp) continue
    const dt = t - prevT
    visit(t, xp - prevXp, dt > ACTIVE_GAP_SEC ? RESUME_SEC : dt)
  }
}

export function activityBetween(points, from, to) {
  let gained = 0
  let activeSec = 0
  forEachGain(points, (t, gain, sec) => {
    if (t <= from || t > to) return
    gained += gain
    activeSec += sec
  })
  return { gained, activeSec }
}

export function activityBuckets(points, now, size, count) {
  const first = localBucketStart(now, size) - (count - 1) * size
  const firstT = points?.[0]?.[0] ?? Infinity
  const buckets = Array.from({ length: count }, (_, i) => {
    const start = first + i * size
    return { start, end: start + size, gained: 0, activeSec: 0, recorded: start + size > firstT, current: start + size > now }
  })
  forEachGain(points, (t, gain, sec) => {
    const bucket = buckets[Math.floor((t - first) / size)]
    if (!bucket || t > now) return
    bucket.gained += gain
    bucket.activeSec += sec
  })
  buckets.forEach((bucket) => {
    bucket.xpStart = xpAt(points, bucket.start) ?? (bucket.recorded ? points[0][1] : null)
    bucket.xpEnd = bucket.recorded ? xpAt(points, Math.min(bucket.end, now)) : null
  })
  return buckets
}

export function lastGainAt(points) {
  for (let i = (points?.length ?? 0) - 1; i > 0; i -= 1) {
    if (points[i][1] > points[i - 1][1]) return points[i][0]
  }
  return null
}

export function activePace(points, now, sec) {
  const sessions = playSessions(points, sec == null ? -Infinity : now - sec)
  const gained = sessions.reduce((sum, session) => sum + session.gained, 0)
  const activeSec = sessions.reduce((sum, session) => sum + session.activeSec, 0)
  if (activeSec < MIN_WINDOW_SEC) return null
  return { gained, activeSec, perHour: (gained / activeSec) * 3600 }
}

export function forecastEta(points, now, remaining) {
  const pace = windowPace(points, now, FORECAST_WINDOW_SEC)
  if (!pace || pace.covered < MIN_FORECAST_SEC || pace.perHour <= 0) return null
  const sec = (remaining / pace.perHour) * 3600
  return {
    sec,
    at: sec <= MAX_FORECAST_SEC ? now + sec : null,
    perDay: pace.perHour * 24,
    basedOnSec: pace.covered,
  }
}

export function formatEta(sec) {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return '—'
  if (sec < 60) return t('units.underMinute')
  if (sec < 3600) return t('units.minutes', { n: Math.round(sec / 60) })
  const hours = sec / 3600
  if (hours < 10) return t('units.hours', { n: hours.toFixed(1) })
  if (hours < 48) return t('units.hours', { n: Math.round(hours) })
  const days = hours / 24
  return t('units.days', { n: days < 10 ? days.toFixed(1) : Math.round(days) })
}

export function formatPlayTime(sec) {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return '—'
  if (sec < 3600) return formatEta(sec)
  const hours = sec / 3600
  return t('units.hours', { n: hours < 10 ? hours.toFixed(1) : formatNumber(Math.round(hours)) })
}

export function formatClock(t) {
  return new Date(t * 1000).toLocaleTimeString(intlLocale(), { hour: '2-digit', minute: '2-digit' })
}

export function formatDay(t) {
  return new Date(t * 1000).toLocaleDateString(intlLocale(), { day: '2-digit', month: '2-digit' })
}

export function formatDate(t) {
  return new Date(t * 1000).toLocaleDateString(intlLocale(), { day: '2-digit', month: '2-digit', year: 'numeric' })
}

export function formatUntil(sec) {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return '—'
  if (sec < 3600) return t('until.underHour')
  if (sec < 2 * 86400) return t('until.hours', { n: Math.round(sec / 3600) })
  if (sec < 60 * 86400) return t('until.days', { n: Math.round(sec / 86400) })
  const months = Math.round(sec / (30 * 86400))
  if (months < 12) return t('until.months', { n: months })
  const years = sec / (365 * 86400)
  const value = years < 10 ? Number(years.toFixed(1)) : Math.round(years)
  return t('until.years', { n: value, value: formatNumber(value) })
}

export function niceTicks(min, max, count = 4) {
  const span = Math.max(max - min, 1)
  const raw = span / count
  const magnitude = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= raw) ?? 10 * magnitude
  const lo = Math.floor(min / step) * step
  const hi = Math.ceil(max / step) * step
  const ticks = []
  for (let v = lo; v <= hi + step / 2; v += step) ticks.push(v)
  return { min: lo, max: hi, ticks }
}

