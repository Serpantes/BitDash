/**
 * XP-over-time math for the skill block. History comes from the backend recorder
 * (/api/xp-history): per skill, [unixSec, cumulativeXp] points, ~1 per minute of play.
 */

/** Pace windows: each is both a stat tile and a chart period. "active" charts play sessions instead of time. */
export const PACE_PERIODS = [
  { id: '15m', label: '15 мин', sec: 15 * 60, bucket: 60 },
  { id: '1h', label: '1 ч', sec: 3600, bucket: 5 * 60 },
  { id: '24h', label: '24 ч', sec: 86400, bucket: 3600 },
  { id: '7d', label: '7 дн', sec: 7 * 86400, bucket: 6 * 3600 },
  { id: '30d', label: '30 дн', sec: 30 * 86400, bucket: 86400 },
  { id: 'active', label: 'В игре', sec: 7 * 86400, bucket: null },
]

const MAX_SESSIONS = 40
// Samples closer than this are continuous play; a longer gap counts as one resumed minute.
const ACTIVE_GAP_SEC = 5 * 60
const RESUME_SEC = 60
const MIN_WINDOW_SEC = 60

/** Splits /api/xp-history samples ({ t, s: { skillId: xp } }) into per-skill point lists. */
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

/** Appends newer points, keeping time and XP monotonic. */
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

/** XP at time t (step function), or null before the first point. */
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

/**
 * XP at time t, linearly interpolated between recorded points (XP grows continuously
 * while playing; across an offline gap the next point only adds ~1 minute of gain, so it stays flat).
 */
export function xpAtInterpolated(points, t) {
  if (!points?.length || t < points[0][0]) return null
  const last = points[points.length - 1]
  if (t >= last[0]) return last[1]
  let lo = 0
  let hi = points.length - 1
  while (hi - lo > 1) {
    const mid = (lo + hi) >> 1
    if (points[mid][0] <= t) lo = mid
    else hi = mid
  }
  const [t0, x0] = points[lo]
  const [t1, x1] = points[hi]
  return t1 > t0 ? x0 + ((t - t0) / (t1 - t0)) * (x1 - x0) : x1
}

/** Calendar pace over the last `sec` seconds (clipped to when recording started). */
export function windowPace(points, now, sec) {
  if (!points?.length) return null
  const firstT = points[0][0]
  const start = Math.max(now - sec, firstT)
  const covered = now - start
  if (covered < MIN_WINDOW_SEC) return null
  const gained = xpAt(points, now) - (xpAt(points, start) ?? points[0][1])
  return { gained, perHour: (gained / covered) * 3600, covered, partial: start > now - sec }
}

/**
 * Play sessions: runs of XP gains with gaps of at most ACTIVE_GAP_SEC.
 * A gain after a longer break counts as one resumed minute of play.
 */
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

/** Recent sessions with their own pace, for the "В игре" chart. */
export function sessionPaces(points, now, sec) {
  return playSessions(points, now - sec)
    .filter((session) => session.activeSec >= MIN_WINDOW_SEC)
    .slice(-MAX_SESSIONS)
    .map((session) => ({ ...session, perHour: (session.gained / session.activeSec) * 3600 }))
}

/** Pace counting only time spent gaining XP — "how fast while actually playing". */
export function activePace(points, now, sec) {
  const sessions = playSessions(points, sec == null ? -Infinity : now - sec)
  const gained = sessions.reduce((sum, session) => sum + session.gained, 0)
  const activeSec = sessions.reduce((sum, session) => sum + session.activeSec, 0)
  if (activeSec < MIN_WINDOW_SEC) return null
  return { gained, activeSec, perHour: (gained / activeSec) * 3600 }
}

/** Local-time aligned buckets of XP gained, e.g. per hour for the last 24h. */
export function paceBuckets(points, from, to, size) {
  if (!points?.length) return []
  const offset = new Date().getTimezoneOffset() * 60
  const firstT = points[0][0]
  const buckets = []
  for (let start = Math.floor((from - offset) / size) * size + offset; start < to; start += size) {
    const end = Math.min(start + size, to)
    if (end <= firstT) {
      buckets.push({ start, end: start + size, gained: null, xpEnd: null })
      continue
    }
    const x0 = xpAt(points, Math.max(start, firstT)) ?? points[0][1]
    const x1 = xpAt(points, end)
    buckets.push({ start, end: start + size, gained: x1 - x0, xpEnd: x1 })
  }
  return buckets
}

/** Evenly spaced cumulative-XP points for the "Всего" line (keeps long periods light). */
export function cumulativeSeries(points, from, to, count = 240) {
  if (!points?.length) return []
  const start = Math.max(from, points[0][0])
  if (to <= start) return []
  const step = (to - start) / (count - 1)
  const series = []
  for (let i = 0; i < count; i += 1) {
    const t = start + step * i
    series.push([t, xpAtInterpolated(points, t) ?? points[0][1]])
  }
  return series
}

export function formatEta(sec) {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return '—'
  if (sec < 60) return '< 1 мин'
  if (sec < 3600) return `${Math.round(sec / 60)} мин`
  const hours = sec / 3600
  if (hours < 10) return `${hours.toFixed(1)} ч`
  if (hours < 48) return `${Math.round(hours)} ч`
  const days = hours / 24
  return days < 10 ? `${days.toFixed(1)} дн` : `${Math.round(days)} дн`
}

/** Play time in hours — "371 ч игры" reads better than "15 дн игры". */
export function formatPlayTime(sec) {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return '—'
  if (sec < 3600) return formatEta(sec)
  const hours = sec / 3600
  return hours < 10 ? `${hours.toFixed(1)} ч` : `${Math.round(hours).toLocaleString('ru-RU')} ч`
}

export function formatClock(t) {
  return new Date(t * 1000).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

export function formatDay(t) {
  return new Date(t * 1000).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' })
}

export function formatBucketRange(bucket, size) {
  if (size < 86400) return `${formatDay(bucket.start)} ${formatClock(bucket.start)}–${formatClock(bucket.end)}`
  if (size === 86400) return formatDay(bucket.start)
  return `${formatDay(bucket.start)}–${formatDay(bucket.end - 1)}`
}

/**
 * SVG path through screen points with monotone cubic smoothing (Fritsch–Carlson, like d3.curveMonotoneX):
 * rounded corners without overshoot, so a never-decreasing XP line never dips.
 */
export function smoothPath(pts) {
  const n = pts.length
  if (n === 0) return ''
  const f = (v) => v.toFixed(1)
  if (n < 3) return pts.map(([x, y], i) => `${i ? 'L' : 'M'}${f(x)},${f(y)}`).join('')
  const dx = []
  const slope = []
  for (let i = 0; i < n - 1; i += 1) {
    dx.push(pts[i + 1][0] - pts[i][0])
    slope.push(dx[i] ? (pts[i + 1][1] - pts[i][1]) / dx[i] : 0)
  }
  const tangent = [slope[0]]
  for (let i = 1; i < n - 1; i += 1) {
    const a = slope[i - 1]
    const b = slope[i]
    tangent.push(a * b <= 0 ? 0 : (3 * (dx[i - 1] + dx[i])) / ((2 * dx[i] + dx[i - 1]) / a + (dx[i] + 2 * dx[i - 1]) / b))
  }
  tangent.push(slope[n - 2])
  let d = `M${f(pts[0][0])},${f(pts[0][1])}`
  for (let i = 0; i < n - 1; i += 1) {
    const h = dx[i] / 3
    const [x0, y0] = pts[i]
    const [x1, y1] = pts[i + 1]
    d += `C${f(x0 + h)},${f(y0 + tangent[i] * h)} ${f(x1 - h)},${f(y1 - tangent[i + 1] * h)} ${f(x1)},${f(y1)}`
  }
  return d
}

/** Round axis ticks covering [min, max] with at most ~`count` steps. */
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

const TIME_TICK_STEPS = [5 * 60, 10 * 60, 15 * 60, 30 * 60, 3600, 3 * 3600, 6 * 3600, 12 * 3600, 86400, 2 * 86400, 5 * 86400, 7 * 86400, 14 * 86400, 30 * 86400]

/** Local-time aligned axis labels: clock for sub-day steps (date at midnight), dates otherwise. */
export function timeTicks(from, to, maxTicks = 6) {
  const offset = new Date().getTimezoneOffset() * 60
  const step = TIME_TICK_STEPS.find((s) => (to - from) / s <= maxTicks) ?? TIME_TICK_STEPS[TIME_TICK_STEPS.length - 1]
  const ticks = []
  for (let t = Math.ceil((from - offset) / step) * step + offset; t <= to; t += step) {
    const midnight = (t - offset) % 86400 === 0
    ticks.push({ t, label: step < 86400 && !midnight ? formatClock(t) : formatDay(t) })
  }
  return ticks
}
