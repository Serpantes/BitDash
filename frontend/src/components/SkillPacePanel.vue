<template>
  <div class="pace">
    <div class="pace-tiles" role="tablist" aria-label="Период графика">
      <button
        v-for="tile in tiles"
        :key="tile.id"
        type="button"
        role="tab"
        class="pace-tile"
        :class="{ 'pace-tile--selected': tile.id === periodDef.id }"
        :aria-selected="tile.id === periodDef.id"
        :title="tile.title"
        @click="$emit('update:period', tile.id)"
      >
        <span class="pace-tile-label">{{ tile.label }}</span>
        <span class="pace-tile-value">{{ tile.value }}<small v-if="tile.value !== '—'">/ч</small></span>
        <span class="pace-tile-sub">{{ tile.sub }}</span>
      </button>
    </div>

    <div class="pace-chart-head">
      <span class="pace-chart-title">{{ chartTitle }}</span>
      <div v-if="!isSessions" class="pace-switch">
        <button type="button" class="pace-switch-btn" :class="{ active: mode !== 'total' }" @click="$emit('update:mode', 'gain')">Прирост</button>
        <button type="button" class="pace-switch-btn" :class="{ active: mode === 'total' }" @click="$emit('update:mode', 'total')">Всего</button>
      </div>
    </div>

    <div class="pace-chart-wrap">
      <svg
        class="pace-svg"
        :viewBox="`0 0 ${BOX.width} ${BOX.height}`"
        role="img"
        :aria-label="chartTitle"
        @mouseleave="hoverIndex = null; hoverT = null"
      >
        <g class="pace-grid">
          <g v-for="tick in yAxis.ticks" :key="`y${tick.value}`">
            <line :x1="plot.left" :x2="plot.right" :y1="tick.y" :y2="tick.y" />
            <text :x="plot.left - 6" :y="tick.y" dy="0.32em" text-anchor="end">{{ tick.label }}</text>
          </g>
          <text
            v-for="tick in xTicks"
            :key="`x${tick.key}`"
            :x="tick.x"
            :y="plot.bottom + 15"
            text-anchor="middle"
          >{{ tick.label }}</text>
        </g>

        <template v-if="hasData && showLine">
          <g v-for="line in levelLines" :key="`l${line.level}`" class="pace-level">
            <line :x1="plot.left" :x2="plot.right" :y1="line.y" :y2="line.y" />
            <text :x="plot.right - 2" :y="line.y - 3" text-anchor="end">ур. {{ line.level }}</text>
          </g>
          <path class="pace-line" :d="linePaths.line" />
          <g v-if="lineHover">
            <line class="pace-crosshair" :x1="lineHover.x" :x2="lineHover.x" :y1="plot.top" :y2="plot.bottom" />
            <circle class="pace-hover-dot" :cx="lineHover.x" :cy="lineHover.y" r="4" />
          </g>
          <circle v-if="linePaths.end" class="pace-end-dot" :cx="linePaths.end.x" :cy="linePaths.end.y" r="4.5" />
          <rect
            class="pace-hit pace-hit--line"
            :x="plot.left"
            :y="plot.top"
            :width="plot.width"
            :height="plot.height"
            @mousemove="onLineMove"
          />
        </template>

        <template v-else-if="hasData">
          <path
            v-for="bar in bars"
            :key="bar.key"
            class="pace-bar"
            :class="{ 'pace-bar--current': bar.current, 'pace-bar--hover': hoverIndex === bar.index }"
            :d="bar.path"
          />
          <rect
            v-for="bar in bars"
            :key="`hit${bar.key}`"
            class="pace-hit"
            :x="bar.bandX"
            :y="plot.top"
            :width="bar.bandW"
            :height="plot.height"
            @mouseenter="hoverIndex = bar.index"
          />
        </template>
      </svg>

      <div v-if="!points?.length" class="pace-empty">
        История опыта записывается с момента добавления блока — первые точки появятся через минуту игры.
      </div>
      <div v-else-if="!hasData" class="pace-empty">
        {{ isSessions ? 'Нет игровых сессий за 7 дней' : 'Нет прироста опыта за этот период' }}
      </div>

      <div
        v-if="tooltip"
        class="pace-tooltip"
        :class="{ 'pace-tooltip--left': tooltip.x > BOX.width * 0.55 }"
        :style="{ left: `${(tooltip.x / BOX.width) * 100}%`, top: `${(tooltip.y / BOX.height) * 100}%` }"
      >
        <div class="pace-tooltip-title">{{ tooltip.title }}</div>
        <div v-for="row in tooltip.rows" :key="row[0]" class="pace-tooltip-row"><span>{{ row[0] }}</span><span>{{ row[1] }}</span></div>
      </div>
    </div>

    <div class="pace-foot">{{ recordingLabel }}</div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'
import { formatXpCompact, formatXpShort, levelForXp } from '../skillProgress'
import {
  PACE_PERIODS,
  activePace,
  cumulativeSeries,
  formatBucketRange,
  formatClock,
  formatDay,
  formatEta,
  formatPlayTime,
  niceTicks,
  paceBuckets,
  sessionPaces,
  smoothPath,
  timeTicks,
  windowPace,
  xpAtInterpolated,
} from '../xpHistory'

const BOX = { width: 420, height: 226, left: 40, right: 8, top: 10, bottom: 20 }
const BUCKET_LABELS = { 60: 'минуту', 300: '5 минут', 3600: 'час', 21600: '6 часов', 86400: 'день' }
const DEFAULT_PERIOD = '24h'
// "Всего" is drawn against the whole goal: 0 → level 100, then 0 → 110 (and 120) once 100 is reached.
const TOTAL_SCALE_LEVELS = [100, 110, 120]

export default {
  name: 'SkillPacePanel',
  props: {
    points: { type: Array, default: null },
    levelTable: { type: Array, required: true },
    now: { type: Number, required: true },
    period: { type: String, default: DEFAULT_PERIOD },
    mode: { type: String, default: 'gain' },
    trackedSince: { type: Number, default: null },
    level: { type: Number, default: 1 },
  },
  emits: ['update:mode', 'update:period'],
  setup(props) {
    const hoverIndex = ref(null)
    const hoverT = ref(null)
    const plot = {
      left: BOX.left,
      right: BOX.width - BOX.right,
      top: BOX.top,
      bottom: BOX.height - BOX.bottom,
      width: BOX.width - BOX.left - BOX.right,
      height: BOX.height - BOX.top - BOX.bottom,
    }

    const periodDef = computed(() => PACE_PERIODS.find((p) => p.id === props.period)
      ?? PACE_PERIODS.find((p) => p.id === DEFAULT_PERIOD))
    const isSessions = computed(() => periodDef.value.bucket == null)
    const showLine = computed(() => !isSessions.value && props.mode === 'total')

    const tiles = computed(() => PACE_PERIODS.map((p) => {
      if (p.bucket == null) {
        const active = activePace(props.points, props.now, p.sec)
        return {
          id: p.id,
          label: p.label,
          value: active ? formatXpShort(active.perHour) : '—',
          sub: active ? formatPlayTime(active.activeSec) : 'нет данных',
          title: active
            ? `Темп только за время, когда опыт рос: ${formatPlayTime(active.activeSec)} игры за 7 дней, без пауз и офлайна`
            : 'Темп только за время, когда опыт рос (последние 7 дней)',
        }
      }
      const pace = windowPace(props.points, props.now, p.sec)
      return {
        id: p.id,
        label: p.label,
        value: pace ? formatXpShort(pace.perHour) : '—',
        sub: !pace ? 'нет данных' : pace.partial ? `за ${formatEta(pace.covered)}` : `+${formatXpShort(pace.gained)}`,
        title: pace
          ? `За ${pace.partial ? `${formatEta(pace.covered)} записи` : p.label}: +${formatXpCompact(pace.gained)} XP, в среднем ${formatXpCompact(pace.perHour)} XP/ч с учётом пауз`
          : 'Мало данных',
      }
    }))

    // Time periods: local-time aligned buckets on a time axis.
    const buckets = computed(() => (isSessions.value
      ? []
      : paceBuckets(props.points, props.now - periodDef.value.sec, props.now, periodDef.value.bucket)))
    const domain = computed(() => {
      const list = buckets.value
      if (!list.length) return { from: props.now - periodDef.value.sec, to: props.now }
      return { from: list[0].start, to: list[list.length - 1].end }
    })
    const x = (t) => plot.left + ((t - domain.value.from) / Math.max(1, domain.value.to - domain.value.from)) * plot.width

    // "В игре": one bar per play session, height = XP/h within it.
    const sessions = computed(() => (isSessions.value ? sessionPaces(props.points, props.now, periodDef.value.sec) : []))

    const hasData = computed(() => (isSessions.value ? sessions.value.length > 0 : buckets.value.some((b) => b.gained > 0)))
    const chartTitle = computed(() => {
      if (isSessions.value) return 'Темп по игровым сессиям, XP/ч'
      if (showLine.value) return 'Опыт навыка'
      return `XP за ${BUCKET_LABELS[periodDef.value.bucket] ?? 'период'}`
    })

    const series = computed(() => (showLine.value
      ? cumulativeSeries(props.points, domain.value.from, Math.min(domain.value.to, props.now))
      : []))

    const yAxis = computed(() => {
      if (!hasData.value) return { min: 0, max: 1, y: () => plot.bottom, ticks: [] }
      if (isSessions.value) return withTickY(niceTicks(0, Math.max(...sessions.value.map((s) => s.perHour)), 4))
      if (showLine.value) {
        const max = props.levelTable[totalScaleLevel.value - 1]
        return withTickY({ min: 0, max, ticks: niceTicks(0, max, 4).ticks.filter((v) => v <= max) })
      }
      return withTickY(niceTicks(0, Math.max(...buckets.value.map((b) => b.gained ?? 0)), 4))
    })
    function withTickY(scale) {
      const y = (v) => plot.bottom - ((v - scale.min) / Math.max(1, scale.max - scale.min)) * plot.height
      return { ...scale, y, ticks: scale.ticks.map((value) => ({ value, y: y(value), label: formatXpCompact(value) })) }
    }

    const xTicks = computed(() => {
      if (!isSessions.value) {
        // Axis stops at "now": the current bucket's end is in the future.
        return timeTicks(domain.value.from, Math.min(domain.value.to, props.now), 6).map((tick) => ({ ...tick, key: tick.t, x: x(tick.t) }))
      }
      const list = sessions.value
      const bandW = plot.width / Math.max(1, list.length)
      const every = Math.ceil(list.length / 6)
      const multiDay = list.length > 1 && formatDay(list[0].start) !== formatDay(list[list.length - 1].start)
      return list
        .map((s, i) => ({ key: s.start, x: plot.left + bandW * (i + 0.5), label: multiDay ? formatDay(s.start) : formatClock(s.start), i }))
        .filter((tick) => (list.length - 1 - tick.i) % every === 0)
    })

    const bars = computed(() => {
      if (isSessions.value) {
        const bandW = plot.width / Math.max(1, sessions.value.length)
        return sessions.value.map((s, index) => barFor(s.perHour, plot.left + bandW * index, bandW, { ...s, key: s.start, index }))
      }
      const bandW = (plot.width * periodDef.value.bucket) / Math.max(1, domain.value.to - domain.value.from)
      return buckets.value.map((b, index) => barFor(b.gained, x(b.start), bandW, { ...b, key: b.start, index, current: b.end > props.now }))
    })
    function barFor(value, bandX, bandW, extra) {
      const barW = Math.max(2, Math.min(24, bandW - 2))
      const height = value > 0 ? Math.max(1, plot.bottom - yAxis.value.y(value)) : 0
      return {
        ...extra,
        bandX,
        bandW,
        path: height ? barPath(bandX + (bandW - barW) / 2, plot.bottom - height, barW, height) : '',
      }
    }
    function barPath(bx, by, w, h) {
      const r = Math.min(4, w / 2, h)
      const base = by + h
      return `M${bx},${base}V${by + r}Q${bx},${by} ${bx + r},${by}H${bx + w - r}Q${bx + w},${by} ${bx + w},${by + r}V${base}Z`
    }

    const linePaths = computed(() => {
      const pts = series.value.map(([t, v]) => [x(t), yAxis.value.y(v)])
      if (!pts.length) return { line: '', end: null }
      const last = pts[pts.length - 1]
      // No area fill: on the 0 → level-100 scale it only draws a wall where recording started.
      return { line: smoothPath(pts), end: { x: last[0], y: last[1] } }
    })
    const totalScaleLevel = computed(() => {
      const cap = props.levelTable.length
      const scale = TOTAL_SCALE_LEVELS.find((level) => props.level < level) ?? cap
      return Math.min(scale, cap)
    })
    // Same marks as the "Путь до 100" bar: top level and 5 / 10 / 20 levels below it.
    const levelLines = computed(() => {
      if (!showLine.value) return []
      const top = totalScaleLevel.value
      return [top - 20, top - 10, top - 5, top]
        .filter((level) => level >= 1)
        .map((level) => ({ level, y: yAxis.value.y(props.levelTable[level - 1]) }))
    })

    const lineHover = computed(() => {
      if (hoverT.value == null || !series.value.length) return null
      const t = Math.min(Math.max(hoverT.value, series.value[0][0]), series.value[series.value.length - 1][0])
      const xp = xpAtInterpolated(props.points, t)
      if (xp == null) return null
      return { t, xp, x: x(t), y: yAxis.value.y(xp) }
    })
    function onLineMove(event) {
      const rect = event.currentTarget.ownerSVGElement?.getBoundingClientRect()
      if (!rect?.width) return
      const px = ((event.clientX - rect.left) / rect.width) * BOX.width
      hoverT.value = domain.value.from + ((px - plot.left) / plot.width) * (domain.value.to - domain.value.from)
    }

    const tooltip = computed(() => {
      if (!hasData.value) return null
      if (showLine.value) {
        const h = lineHover.value
        if (!h) return null
        return {
          x: h.x,
          y: Math.min(h.y, plot.bottom - 30),
          title: `${formatDay(h.t)} ${formatClock(h.t)}`,
          rows: [['Опыт', `${formatXpCompact(h.xp)} XP`], ['Уровень', String(levelForXp(props.levelTable, h.xp))]],
        }
      }
      const bar = bars.value[hoverIndex.value]
      if (!bar) return null
      const at = { x: bar.bandX + bar.bandW / 2, y: plot.top + plot.height * 0.3 }
      if (isSessions.value) {
        return {
          ...at,
          title: `${formatDay(bar.start)} ${formatClock(bar.start)}–${formatClock(bar.end)}`,
          rows: [
            ['Игра', formatPlayTime(bar.activeSec)],
            ['Прирост', `+${formatXpCompact(bar.gained)} XP`],
            ['Темп', `${formatXpCompact(bar.perHour)} XP/ч`],
          ],
        }
      }
      if (bar.gained == null) return null
      const spanSec = Math.min(bar.end, props.now) - Math.max(bar.start, props.points[0][0])
      const rows = [['Прирост', `+${formatXpCompact(bar.gained)} XP`]]
      if (spanSec > 0) rows.push(['Темп', `${formatXpCompact((bar.gained / spanSec) * 3600)} XP/ч`])
      const fromLevel = levelForXp(props.levelTable, bar.xpEnd - bar.gained)
      const toLevel = levelForXp(props.levelTable, bar.xpEnd)
      rows.push(['Уровень', fromLevel === toLevel ? String(toLevel) : `${fromLevel} → ${toLevel}`])
      return { ...at, title: `${formatBucketRange(bar, periodDef.value.bucket)}${bar.current ? ' (идёт)' : ''}`, rows }
    })

    const recordingLabel = computed(() => {
      const since = props.trackedSince ?? props.points?.[0]?.[0]
      return since ? `Запись с ${formatDay(since)} ${formatClock(since)}` : 'Запись опыта начата'
    })

    return {
      BOX, plot, hoverIndex, hoverT, periodDef, isSessions, showLine, tiles, hasData, chartTitle, yAxis, xTicks, bars,
      linePaths, levelLines, lineHover, onLineMove, tooltip, recordingLabel,
    }
  },
}
</script>

<style scoped>
.pace { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 8px; }
.pace-tiles { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 5px; }
.pace-tile {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  min-width: 0;
  padding: 5px 6px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-raised);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.pace-tile:hover { border-color: var(--border-strong); }
.pace-tile--selected, .pace-tile--selected:hover { border-color: var(--accent); background: rgba(138, 164, 255, 0.14); }
.pace-tile-label { font-size: 10px; font-weight: 700; letter-spacing: 0.03em; text-transform: uppercase; color: var(--muted); white-space: nowrap; }
.pace-tile--selected .pace-tile-label { color: var(--accent); }
.pace-tile-value { font-size: 14px; font-weight: 700; color: var(--text); white-space: nowrap; }
.pace-tile-value small { margin-left: 1px; font-size: 9px; font-weight: 600; color: var(--muted); }
.pace-tile-sub { max-width: 100%; font-size: 10px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-variant-numeric: tabular-nums; }

.pace-chart-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; min-height: 20px; }
.pace-chart-title { font-size: 12px; font-weight: 700; color: var(--text-soft); }
.pace-switch { display: inline-flex; border: 1px solid var(--border); border-radius: 7px; overflow: hidden; }
.pace-switch-btn { border: none; background: var(--surface-raised); color: var(--muted); font-size: 10px; font-weight: 600; padding: 2px 7px; cursor: pointer; }
.pace-switch-btn.active { background: rgba(138, 164, 255, 0.2); color: var(--accent); }

.pace-chart-wrap { position: relative; flex: 1; min-height: 0; }
.pace-svg { display: block; width: 100%; height: auto; max-height: 100%; overflow: visible; }
.pace-grid line { stroke: var(--border); stroke-width: 1; opacity: 0.55; }
.pace-grid text { fill: var(--muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.pace-bar { fill: var(--accent); }
.pace-bar--current { opacity: 0.55; }
.pace-bar--hover { fill: var(--text-soft); opacity: 1; }
.pace-hit { fill: transparent; }
.pace-hit--line { cursor: crosshair; }
.pace-level line { stroke: var(--accent); stroke-width: 1; opacity: 0.3; }
.pace-level text { fill: var(--muted); font-size: 9px; font-weight: 600; }
.pace-line { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.pace-end-dot { fill: var(--accent); stroke: var(--surface); stroke-width: 2; }
.pace-hover-dot { fill: var(--text-soft); stroke: var(--surface); stroke-width: 2; }
.pace-crosshair { stroke: var(--text-soft); stroke-width: 1; opacity: 0.35; }
.pace-empty {
  position: absolute;
  inset: 12px 12px 26px 44px;
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
.pace-tooltip {
  position: absolute;
  z-index: 2;
  min-width: 140px;
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-raised);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.35);
  transform: translate(12px, -50%);
  pointer-events: none;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.pace-tooltip--left { transform: translate(calc(-100% - 12px), -50%); }
.pace-tooltip-title { font-size: 12px; font-weight: 700; color: var(--text); margin-bottom: 3px; white-space: nowrap; }
.pace-tooltip-row { display: flex; justify-content: space-between; gap: 12px; color: var(--muted); white-space: nowrap; }
.pace-tooltip-row span:last-child { color: var(--text-soft); font-weight: 600; }
.pace-foot { font-size: 11px; color: var(--muted); }
</style>
