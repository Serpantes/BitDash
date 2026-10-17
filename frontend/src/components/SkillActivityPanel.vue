<template>
  <div class="activity">
    <div class="activity-kpis">
      <div v-for="kpi in kpis" :key="kpi.id" class="activity-kpi" :title="kpi.title">
        <span class="activity-kpi-label">{{ kpi.label }}</span>
        <span class="activity-kpi-value">{{ kpi.value }}<small v-if="kpi.unit">{{ kpi.unit }}</small></span>
        <span class="activity-kpi-sub" :class="{ 'activity-kpi-sub--live': kpi.live }">{{ kpi.sub }}</span>
      </div>
    </div>

    <div class="activity-head">
      <span class="activity-title">{{ periodDef.bucket < 86400 ? t('activity.xpByHour') : t('activity.xpByDay') }}</span>
      <span v-if="levelUps.length" class="activity-legend"><span class="activity-legend-mark" aria-hidden="true"></span>{{ t('activity.newLevel') }}</span>
      <div class="bc-seg" role="tablist" :aria-label="t('activity.period')">
        <button
          v-for="p in ACTIVITY_PERIODS"
          :key="p.id"
          type="button"
          role="tab"
          class="bc-seg-btn"
          :class="{ active: p.id === periodDef.id }"
          :aria-selected="p.id === periodDef.id"
          @click="$emit('update:period', p.id)"
        >{{ p.label }}</button>
      </div>
    </div>

    <div class="activity-chart">
      <svg
        class="activity-svg"
        :viewBox="`0 0 ${BOX.width} ${BOX.height}`"
        role="img"
        :aria-label="chartLabel"
        @mouseleave="hoverIndex = null"
      >
        <rect
          v-if="unrecorded"
          class="activity-unrecorded"
          :x="plot.left"
          :y="plot.top"
          :width="unrecorded.width"
          :height="plot.height"
        />
        <text
          v-if="unrecorded?.label"
          class="activity-unrecorded-label"
          :x="plot.left + unrecorded.width / 2"
          :y="plot.top + plot.height / 2"
          text-anchor="middle"
          dy="0.32em"
        >{{ t('activity.notRecorded') }}</text>

        <g class="activity-grid">
          <g v-for="tick in yAxis.ticks" :key="`y${tick.value}`">
            <line :x1="plot.left" :x2="plot.right" :y1="tick.y" :y2="tick.y" />
            <text :x="plot.left - 6" :y="tick.y" dy="0.32em" text-anchor="end">{{ tick.label }}</text>
          </g>
        </g>

        <path
          v-for="bar in bars"
          :key="bar.start"
          class="activity-bar"
          :class="{ 'activity-bar--current': bar.current, 'activity-bar--hover': hoverIndex === bar.index }"
          :d="bar.path"
        />

        <text
          v-for="label in valueLabels"
          :key="`v${label.index}`"
          class="activity-value"
          :x="label.x"
          :y="label.y"
          text-anchor="middle"
        >{{ label.text }}</text>

        <path v-for="mark in levelUps" :key="`l${mark.index}`" class="activity-levelup" :d="mark.path" />

        <g class="activity-grid">
          <text
            v-for="tick in xTicks"
            :key="`x${tick.index}`"
            :x="tick.x"
            :y="plot.bottom + 22"
            text-anchor="middle"
          >{{ tick.label }}</text>
        </g>

        <rect
          v-for="bar in bars"
          :key="`hit${bar.start}`"
          class="activity-hit"
          :x="bar.bandX"
          :y="plot.top"
          :width="bar.bandW"
          :height="plot.height + 12"
          tabindex="0"
          @mouseenter="hoverIndex = bar.index"
          @focus="hoverIndex = bar.index"
          @blur="hoverIndex = null"
        />
      </svg>

      <div v-if="!points?.length" class="activity-empty">
        {{ t('activity.noHistory') }}
      </div>
      <div v-else-if="!hasGain" class="activity-empty">{{ t('activity.noGain') }}</div>

      <div
        v-if="tooltip"
        class="bc-tooltip activity-tooltip"
        :class="{ 'activity-tooltip--left': tooltip.x > BOX.width * 0.55 }"
        :style="{ left: `${(tooltip.x / BOX.width) * 100}%`, top: `${(tooltip.y / BOX.height) * 100}%` }"
      >
        <div class="activity-tooltip-title">{{ tooltip.title }}</div>
        <div v-for="row in tooltip.rows" :key="row[0]" class="activity-tooltip-row"><span>{{ row[0] }}</span><span>{{ row[1] }}</span></div>
      </div>
    </div>

    <div class="activity-foot">{{ recordingLabel }}</div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'
import { formatXpCompact, formatXpShort, levelForXp } from '../skillProgress'
import {
  ACTIVITY_PERIODS,
  activePace,
  activityBetween,
  activityBuckets,
  activityPeriod,
  formatClock,
  formatDay,
  formatPlayTime,
  lastGainAt,
  localBucketStart,
  niceTicks,
} from '../xpHistory'
import { formatNumber, intlLocale, t } from '../i18n'

const BOX = { width: 420, height: 196, left: 40, right: 8, top: 18, bottom: 30 }
const WEEK_SEC = 7 * 86400
const LIVE_SEC = 3 * 60
const NOW_PACE_SEC = 15 * 60

function formatAgo(sec) {
  if (sec < 3600) return t('units.minutes', { n: Math.max(1, Math.round(sec / 60)) })
  if (sec < 48 * 3600) return t('units.hours', { n: Math.round(sec / 3600) })
  return t('units.days', { n: Math.round(sec / 86400) })
}

function weekday(t) {
  return new Date(t * 1000).toLocaleDateString(intlLocale(), { weekday: 'short' })
}

export default {
  name: 'SkillActivityPanel',
  props: {
    points: { type: Array, default: null },
    levelTable: { type: Array, required: true },
    now: { type: Number, required: true },
    period: { type: String, default: '7d' },
    trackedSince: { type: Number, default: null },
  },
  emits: ['update:period'],
  setup(props) {
    const hoverIndex = ref(null)
    const plot = {
      left: BOX.left,
      right: BOX.width - BOX.right,
      top: BOX.top,
      bottom: BOX.height - BOX.bottom,
      width: BOX.width - BOX.left - BOX.right,
      height: BOX.height - BOX.top - BOX.bottom,
    }
    const periodDef = computed(() => activityPeriod(props.period))
    const firstT = computed(() => props.points?.[0]?.[0] ?? null)

    const kpis = computed(() => {
      const points = props.points
      const now = props.now
      const today = activityBetween(points, localBucketStart(now, 86400), now)
      const week = activityBetween(points, now - WEEK_SEC, now)
      const weekCovered = firstT.value == null ? 0 : Math.min(WEEK_SEC, now - firstT.value)
      const pace = activePace(points, now, WEEK_SEC) ?? activePace(points, now, null)
      const lastGain = lastGainAt(points)
      const live = lastGain != null && now - lastGain <= LIVE_SEC
      const nowPace = live ? activePace(points, now, NOW_PACE_SEC) : null
      const played = (activity) => (activity.activeSec >= 60 ? t('activity.played', { time: formatPlayTime(activity.activeSec) }) : t('activity.notPlayed'))
      return [
        {
          id: 'today',
          label: t('activity.today'),
          value: `+${formatXpShort(today.gained)}`,
          unit: ' XP',
          sub: played(today),
          title: t('activity.todayTitle', { xp: formatXpCompact(today.gained), time: formatPlayTime(today.activeSec) }),
        },
        {
          id: 'week',
          label: t('activity.week'),
          value: `+${formatXpShort(week.gained)}`,
          unit: ' XP',
          sub: played(week),
          title: weekCovered < WEEK_SEC - 3600
            ? t('activity.weekPartial', { span: formatAgo(weekCovered) })
            : t('activity.weekTitle', { xp: formatXpCompact(week.gained), time: formatPlayTime(week.activeSec) }),
        },
        {
          id: 'pace',
          label: t('activity.pace'),
          value: pace ? formatXpShort(pace.perHour) : '—',
          unit: pace ? t('activity.perHour') : '',
          sub: live && nowPace
            ? t('activity.nowPace', { xp: formatXpShort(nowPace.perHour) })
            : lastGain != null ? t('activity.lastPlayed', { span: formatAgo(now - lastGain) }) : t('common.noData'),
          live: live && Boolean(nowPace),
          title: pace
            ? t('activity.paceTitle')
            : t('activity.paceEmpty'),
        },
      ]
    })

    const buckets = computed(() => activityBuckets(props.points, props.now, periodDef.value.bucket, periodDef.value.count))
    const hasGain = computed(() => buckets.value.some((b) => b.gained > 0))
    const bandW = computed(() => plot.width / periodDef.value.count)
    const yAxis = computed(() => {
      const max = Math.max(0, ...buckets.value.map((b) => b.gained))
      const scale = niceTicks(0, max || 1, 3)
      const y = (v) => plot.bottom - (v / Math.max(1, scale.max)) * plot.height
      return { y, max: scale.max, ticks: scale.ticks.map((value) => ({ value, y: y(value), label: formatXpCompact(value) })) }
    })
    const bars = computed(() => buckets.value.map((bucket, index) => {
      const bandX = plot.left + bandW.value * index
      const barW = Math.max(2, bandW.value - 2)
      const height = bucket.gained > 0 ? Math.max(2, plot.bottom - yAxis.value.y(bucket.gained)) : 0
      const x = bandX + (bandW.value - barW) / 2
      return {
        ...bucket,
        index,
        bandX,
        bandW: bandW.value,
        cx: bandX + bandW.value / 2,
        top: plot.bottom - height,
        path: height ? barPath(x, plot.bottom - height, barW, height) : '',
      }
    }))
    function barPath(x, y, w, h) {
      const r = Math.min(4, w / 2, h)
      const base = y + h
      return `M${x},${base}V${y + r}Q${x},${y} ${x + r},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${base}Z`
    }

    const unrecorded = computed(() => {
      const firstRecorded = bars.value.findIndex((bar) => bar.recorded)
      if (firstRecorded <= 0 || !props.points?.length) return null
      const width = bandW.value * firstRecorded
      return { width, label: width >= 64 }
    })

    const valueLabels = computed(() => {
      const withGain = bars.value.filter((bar) => bar.gained > 0)
      if (!withGain.length) return []
      const best = withGain.reduce((a, b) => (b.gained > a.gained ? b : a))
      const current = bars.value[bars.value.length - 1]
      const picked = [best]
      if (current.gained > 0 && current !== best && Math.abs(current.cx - best.cx) > 40) picked.push(current)
      return picked.map((bar) => ({ index: bar.index, x: bar.cx, y: bar.top - 5, text: formatXpShort(bar.gained) }))
    })

    const levelUps = computed(() => bars.value
      .filter((bar) => bar.xpStart != null && bar.xpEnd != null
        && levelForXp(props.levelTable, bar.xpEnd) > levelForXp(props.levelTable, bar.xpStart))
      .map((bar) => {
        const cx = bar.cx
        const cy = plot.bottom + 8
        return { index: bar.index, path: `M${cx},${cy - 4}L${cx + 4},${cy}L${cx},${cy + 4}L${cx - 4},${cy}Z` }
      }))

    const xTicks = computed(() => {
      const hourly = periodDef.value.bucket < 86400
      const count = bars.value.length
      const shown = (bar) => (hourly
        ? new Date(bar.start * 1000).getHours() % 6 === 0
        : count <= 8 || (count - 1 - bar.index) % 5 === 0)
      return bars.value
        .filter(shown)
        .map((bar) => ({
          index: bar.index,
          x: bar.cx,
          label: hourly ? formatClock(bar.start) : count <= 8 ? `${weekday(bar.start)} ${String(new Date(bar.start * 1000).getDate()).padStart(2, '0')}` : formatDay(bar.start),
        }))
    })

    const tooltip = computed(() => {
      const bar = bars.value[hoverIndex.value]
      if (!bar) return null
      const hourly = periodDef.value.bucket < 86400
      const when = hourly
        ? `${formatDay(bar.start)} ${formatClock(bar.start)}–${formatClock(bar.end)}`
        : `${weekday(bar.start)}, ${formatDay(bar.start)}`
      const title = `${when}${bar.current ? ` · ${hourly ? t('activity.now') : t('activity.todayLower')}` : ''}`
      const at = { x: bar.cx, y: plot.top + plot.height * 0.35 }
      if (!bar.recorded) return { ...at, title, rows: [[t('activity.data'), t('activity.notRecorded')]] }
      const rows = [[t('activity.xp'), `+${formatNumber(Math.round(bar.gained))} XP`]]
      if (bar.activeSec >= 60) {
        rows.push([t('activity.play'), formatPlayTime(bar.activeSec)])
        rows.push([t('activity.tempo'), t('activity.xpPerHour', { xp: formatXpShort((bar.gained / bar.activeSec) * 3600) })])
      }
      if (bar.xpStart != null && bar.xpEnd != null) {
        const from = levelForXp(props.levelTable, bar.xpStart)
        const to = levelForXp(props.levelTable, bar.xpEnd)
        rows.push([t('skillCard.level'), from === to ? String(to) : `${from} → ${to}`])
      }
      return { ...at, title, rows }
    })

    const chartLabel = computed(() => {
      const total = buckets.value.reduce((sum, b) => sum + b.gained, 0)
      return t('activity.chartAria', { kind: periodDef.value.bucket < 86400 ? t('activity.xpByHour') : t('activity.xpByDay'), period: periodDef.value.label, xp: formatXpCompact(total) })
    })

    const recordingLabel = computed(() => {
      const since = props.trackedSince ?? firstT.value
      return since ? t('activity.recordingSince', { when: `${formatDay(since)} ${formatClock(since)}` }) : t('activity.recordingStarted')
    })

    return {
      ACTIVITY_PERIODS, BOX, plot, hoverIndex, periodDef, kpis, hasGain, yAxis, bars, unrecorded,
      valueLabels, levelUps, xTicks, tooltip, chartLabel, recordingLabel,
    }
  },
}
</script>

<style scoped>
.activity { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }

.activity-kpis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
.activity-kpi {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  padding: 6px 9px 7px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.3);
}
.activity-kpi-label {
  font-family: var(--font-title);
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted);
  white-space: nowrap;
}
.activity-kpi-value { font-size: 18px; font-weight: 800; line-height: 1.15; color: var(--text); white-space: nowrap; }
.activity-kpi-value small { font-size: 11px; font-weight: 700; color: var(--muted); }
.activity-kpi-sub { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.activity-kpi-sub--live { color: var(--text-soft); }
.activity-kpi-sub--live::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  margin: 0 5px 1px 0;
  border-radius: 50%;
  background: var(--success);
  vertical-align: middle;
}

.activity-head { display: flex; align-items: center; gap: 10px; min-height: 22px; }
.activity-title { font-size: 12px; font-weight: 800; color: var(--text-soft); }
.activity-legend { display: inline-flex; align-items: center; gap: 5px; font-size: 11px; color: var(--muted); }
.activity-legend-mark { width: 7px; height: 7px; background: var(--gold); transform: rotate(45deg); }
.activity-head .bc-seg { margin-left: auto; }

.activity-chart { position: relative; flex: 1; min-height: 0; }
.activity-svg { display: block; width: 100%; height: auto; max-height: 100%; overflow: visible; }
.activity-grid line { stroke: var(--border); stroke-width: 1; opacity: 0.6; }
.activity-grid text { fill: var(--muted); font-size: 10px; font-weight: 600; font-variant-numeric: tabular-nums; }
.activity-unrecorded { fill: rgba(233, 223, 196, 0.035); }
.activity-unrecorded-label { fill: var(--muted); font-size: 10px; font-style: italic; }
.activity-bar { fill: var(--accent); transition: filter 0.1s; }
.activity-bar--current { opacity: 0.55; }
.activity-bar--hover { filter: brightness(1.25); opacity: 1; }
.activity-value { fill: var(--text); font-size: 10px; font-weight: 800; }
.activity-levelup { fill: var(--gold); stroke: var(--panel); stroke-width: 2; paint-order: stroke; }
.activity-hit { fill: transparent; outline: none; }
.activity-hit:focus-visible { fill: rgba(233, 223, 196, 0.06); }
.activity-empty {
  position: absolute;
  inset: 12px 12px 30px 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  font-size: 12px;
  line-height: 1.4;
  color: var(--muted);
  font-style: italic;
  pointer-events: none;
}
.activity-tooltip { transform: translate(12px, -50%); }
.activity-tooltip--left { transform: translate(calc(-100% - 12px), -50%); }
.activity-tooltip-title { font-size: 12px; font-weight: 800; color: var(--text); margin-bottom: 3px; white-space: nowrap; }
.activity-tooltip-row { display: flex; justify-content: space-between; gap: 12px; color: var(--muted); white-space: nowrap; }
.activity-tooltip-row span:last-child { color: var(--text); font-weight: 700; font-variant-numeric: tabular-nums; }
.activity-foot { font-size: 11px; color: var(--muted); }
</style>
