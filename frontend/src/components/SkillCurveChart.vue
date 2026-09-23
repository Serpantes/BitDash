<template>
  <div class="skill-curve">
    <div class="skill-chart-wrap">
      <svg
        class="skill-chart-svg"
        :viewBox="`0 0 ${CHART_BOX.width} ${CHART_BOX.height}`"
        role="img"
        :aria-label="chartLabel"
      >
        <g class="skill-grid">
          <g v-for="tick in chart.yTicks" :key="`y${tick.value}`">
            <line :x1="chart.plot.left" :x2="chart.plot.right" :y1="tick.y" :y2="tick.y" />
            <text :x="chart.plot.left - 6" :y="tick.y" dy="0.32em" text-anchor="end">{{ formatXpCompact(tick.value) }}</text>
          </g>
          <text
            v-for="tick in chart.xTicks"
            :key="`x${tick.level}`"
            :x="tick.x"
            :y="chart.plot.bottom + 15"
            text-anchor="middle"
          >{{ tick.level }}</text>
        </g>

        <path class="skill-curve-rest" :d="chart.linePath" />
        <path class="skill-curve-area" :d="chart.doneAreaPath" />
        <path class="skill-curve-done" :d="chart.doneLinePath" />

        <line class="skill-guide" :x1="chart.plot.left" :x2="chart.dot.x" :y1="chart.dot.y" :y2="chart.dot.y" />
        <line class="skill-guide" :x1="chart.dot.x" :x2="chart.dot.x" :y1="chart.dot.y" :y2="chart.plot.bottom" />

        <template v-if="!progress.reachedTarget">
          <circle class="skill-target-dot" :cx="chart.target.x" :cy="chart.target.y" r="4" />
          <text class="skill-chart-note" :x="chart.target.x - 9" :y="chart.target.y" dy="0.32em" text-anchor="end">
            ур. {{ progress.targetLevel }} · {{ formatXpCompact(progress.targetXp) }}
          </text>
        </template>

        <text
          class="skill-chart-note skill-chart-note--you"
          :x="dotLabelAnchor === 'end' ? chart.dot.x - 9 : chart.dot.x + 9"
          :y="chart.dot.y - 9"
          :text-anchor="dotLabelAnchor"
        >вы · {{ formatXpCompact(progress.xp) }}</text>

        <g v-if="hover">
          <line class="skill-crosshair" :x1="hover.x" :x2="hover.x" :y1="chart.plot.top" :y2="chart.plot.bottom" />
          <circle class="skill-hover-dot" :cx="hover.x" :cy="hover.y" r="4" />
        </g>

        <circle class="skill-you-dot" :cx="chart.dot.x" :cy="chart.dot.y" r="5" />

        <rect
          class="skill-hit"
          :x="chart.plot.left"
          :y="chart.plot.top"
          :width="chart.plot.width"
          :height="chart.plot.bottom - chart.plot.top"
          @mousemove="onChartMove"
          @mouseleave="hoverLevel = null"
        />
      </svg>
      <div
        v-if="hover"
        class="skill-tooltip"
        :class="{ 'skill-tooltip--left': hover.x > CHART_BOX.width * 0.55 }"
        :style="{ left: `${(hover.x / CHART_BOX.width) * 100}%`, top: `${tooltipTopPct}%` }"
      >
        <div class="skill-tooltip-title">Уровень {{ hover.level }}</div>
        <div class="skill-tooltip-row"><span>Всего</span><span>{{ formatXpCompact(hover.total) }} XP</span></div>
        <div v-if="hover.level > 1" class="skill-tooltip-row"><span>Цена уровня</span><span>{{ formatXpCompact(hover.step) }} XP</span></div>
        <div class="skill-tooltip-row">
          <template v-if="hover.remaining > 0"><span>Вам осталось</span><span>{{ formatXpCompact(hover.remaining) }}</span></template>
          <template v-else><span>Пройден</span><span>✓</span></template>
        </div>
      </div>
    </div>
    <div class="skill-chart-foot">
      Каждый уровень дороже предыдущего на ~{{ facts.growthPct.toFixed(1) }}% ·
      больше половины опыта до {{ progress.targetLevel }} — последние {{ facts.halfLevels }} {{ levelsWord(facts.halfLevels) }}
    </div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'
import { curveFacts, formatXpCompact, skillCurveGeometry } from '../skillProgress'

const CHART_BOX = { width: 420, height: 270, left: 42, right: 14, top: 18, bottom: 22 }

export default {
  name: 'SkillCurveChart',
  props: {
    levelTable: { type: Array, required: true },
    progress: { type: Object, required: true },
  },
  setup(props) {
    const hoverLevel = ref(null)
    const chart = computed(() => skillCurveGeometry(props.levelTable, props.progress, CHART_BOX))
    const facts = computed(() => curveFacts(props.levelTable, props.progress.targetLevel))
    const dotLabelAnchor = computed(() => (chart.value.dot.x - chart.value.plot.left > 90 ? 'end' : 'start'))
    const hover = computed(() => {
      const level = hoverLevel.value
      if (!level) return null
      const table = props.levelTable
      const total = table[level - 1]
      return {
        level,
        total,
        step: level > 1 ? total - table[level - 2] : 0,
        remaining: total - props.progress.xp,
        x: chart.value.x(level),
        y: chart.value.y(total),
      }
    })
    // Keep the tooltip inside the plot: the empty area above the convex curve.
    const tooltipTopPct = computed(() => {
      if (!hover.value) return 0
      const pct = (hover.value.y / CHART_BOX.height) * 100
      return Math.min(55, Math.max(22, pct))
    })
    const chartLabel = computed(() =>
      `Кривая опыта до ${chart.value.domainMax} уровня; игрок на уровне ${props.progress.level} с ${formatXpCompact(props.progress.xp)} XP`)

    function onChartMove(event) {
      const rect = event.currentTarget.ownerSVGElement?.getBoundingClientRect()
      if (!rect?.width) return
      hoverLevel.value = chart.value.levelAtX(((event.clientX - rect.left) / rect.width) * CHART_BOX.width)
    }
    function levelsWord(n) {
      const mod10 = n % 10
      const mod100 = n % 100
      if (mod10 === 1 && mod100 !== 11) return 'уровень'
      if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'уровня'
      return 'уровней'
    }

    return { CHART_BOX, hoverLevel, chart, facts, dotLabelAnchor, hover, tooltipTopPct, chartLabel, onChartMove, levelsWord, formatXpCompact }
  },
}
</script>

<style scoped>
.skill-curve { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 6px; }
.skill-chart-wrap { position: relative; flex: 1; min-height: 0; }
.skill-chart-svg { display: block; width: 100%; height: auto; max-height: 100%; overflow: visible; }
.skill-grid line { stroke: var(--border); stroke-width: 1; opacity: 0.55; }
.skill-grid text { fill: var(--muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.skill-curve-rest { fill: none; stroke: var(--border-strong); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.skill-curve-area { fill: var(--accent); opacity: 0.14; }
.skill-curve-done { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.skill-guide { stroke: var(--accent); stroke-width: 1; opacity: 0.4; }
.skill-target-dot { fill: var(--surface); stroke: var(--border-strong); stroke-width: 2; }
.skill-you-dot { fill: var(--accent); stroke: var(--surface); stroke-width: 2; }
.skill-hover-dot { fill: var(--text-soft); stroke: var(--surface); stroke-width: 2; }
.skill-crosshair { stroke: var(--text-soft); stroke-width: 1; opacity: 0.35; }
.skill-chart-note { fill: var(--muted); font-size: 10px; font-weight: 600; }
.skill-chart-note--you { fill: var(--text); }
.skill-hit { fill: transparent; cursor: crosshair; }
.skill-tooltip {
  position: absolute;
  z-index: 2;
  min-width: 150px;
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
.skill-tooltip--left { transform: translate(calc(-100% - 12px), -50%); }
.skill-tooltip-title { font-size: 12px; font-weight: 700; color: var(--text); margin-bottom: 3px; }
.skill-tooltip-row { display: flex; justify-content: space-between; gap: 12px; color: var(--muted); white-space: nowrap; }
.skill-tooltip-row span:last-child { color: var(--text-soft); font-weight: 600; }
.skill-chart-foot { font-size: 11px; line-height: 1.35; color: var(--muted); }
</style>
