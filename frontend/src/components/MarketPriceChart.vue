<template>
  <div ref="wrapRef" class="mpc">
    <svg
      v-if="geo"
      class="mpc-svg"
      :viewBox="`0 0 ${width} ${HEIGHT}`"
      :width="width"
      :height="HEIGHT"
      role="img"
      :aria-label="label"
      tabindex="0"
      @keydown="onKeydown"
      @blur="hoverIndex = null"
    >
      <g class="mpc-grid">
        <template v-for="tick in geo.ticks" :key="tick.value">
          <line :x1="geo.left" :x2="width - PAD.right" :y1="tick.y" :y2="tick.y" />
          <text :x="geo.left - 6" :y="tick.y" dy="0.32em" text-anchor="end">{{ formatPrice(tick.value) }}</text>
        </template>
        <text :x="geo.left" :y="HEIGHT - 3" text-anchor="start">{{ geo.firstDay }}</text>
        <text :x="width - PAD.right" :y="HEIGHT - 3" text-anchor="end">{{ geo.lastDay }}</text>
      </g>
      <path class="mpc-area" :d="geo.area" />
      <path class="mpc-line" :d="geo.line" />
      <g v-if="hover">
        <line class="mpc-crosshair" :x1="hover.x" :x2="hover.x" :y1="PAD.top" :y2="geo.bottom" />
        <circle class="mpc-hover-dot" :cx="hover.x" :cy="hover.y" r="4" />
      </g>
      <circle v-else class="mpc-end" :cx="geo.end.x" :cy="geo.end.y" r="4" />
      <rect
        class="mpc-hit"
        :x="geo.left"
        :y="0"
        :width="Math.max(0, width - PAD.right - geo.left)"
        :height="geo.bottom"
        @pointermove="onPointerMove"
        @pointerleave="hoverIndex = null"
      />
    </svg>
    <div
      v-if="hover"
      class="bc-tooltip mpc-tooltip"
      :class="{ 'mpc-tooltip--left': hover.x > width * 0.6 }"
      :style="{ left: `${hover.x}px` }"
    >
      <div class="mpc-tooltip-title">{{ formatDay(hover.point.t) }}</div>
      <div class="mpc-tooltip-row"><span>{{ t('priceChart.avgPrice') }}</span><span>{{ formatPrice(hover.point.price) }}</span></div>
      <div class="mpc-tooltip-row"><span>{{ t('priceChart.volume') }}</span><span>{{ formatAmount(hover.point.volume) }} {{ t('common.pcs') }}</span></div>
      <div class="mpc-tooltip-row"><span>{{ t('priceChart.trades') }}</span><span>{{ formatAmount(hover.point.trades) }}</span></div>
    </div>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { formatAmount, formatPrice } from '../marketOrders'
import { intlLocale, t } from '../i18n'

const HEIGHT = 104
const PAD = { top: 8, right: 8, bottom: 18 }

function formatDay(ms) {
  return new Date(ms).toLocaleDateString(intlLocale(), { day: '2-digit', month: '2-digit' })
}

function niceStep(range, count) {
  const raw = range / count
  const magnitude = 10 ** Math.floor(Math.log10(raw))
  const norm = raw / magnitude
  return (norm < 1.5 ? 1 : norm < 3 ? 2 : norm < 7 ? 5 : 10) * magnitude
}

export default {
  name: 'MarketPriceChart',
  props: {
    points: { type: Array, required: true },
  },
  setup(props) {
    const wrapRef = ref(null)
    const width = ref(0)
    const hoverIndex = ref(null)
    let observer = null

    onMounted(() => {
      observer = new ResizeObserver(([entry]) => { width.value = Math.floor(entry.contentRect.width) })
      if (wrapRef.value) observer.observe(wrapRef.value)
    })
    onUnmounted(() => observer?.disconnect())

    const geo = computed(() => {
      const points = props.points
      if (!width.value || points.length < 2) return null
      let min = Math.min(...points.map((p) => p.price))
      let max = Math.max(...points.map((p) => p.price))
      if (max - min < 1e-9) {
        const pad = Math.max(1, max * 0.1)
        min -= pad
        max += pad
      }
      const step = niceStep(max - min, 3)
      const lo = Math.max(0, Math.floor(min / step) * step)
      const hi = Math.ceil(max / step) * step
      const ticks = []
      for (let v = lo; v <= hi + step / 2; v += step) ticks.push(v)
      const left = 10 + Math.max(...ticks.map((v) => formatPrice(v).length)) * 6
      const bottom = HEIGHT - PAD.bottom
      const t0 = points[0].t
      const t1 = points[points.length - 1].t
      const x = (t) => left + ((t - t0) / Math.max(1, t1 - t0)) * (width.value - PAD.right - left)
      const y = (v) => bottom - ((v - lo) / (hi - lo)) * (bottom - PAD.top)
      const coords = points.map((p) => ({ x: x(p.t), y: y(p.price) }))
      const line = coords.map((c, i) => `${i ? 'L' : 'M'}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join('')
      const area = `${line}L${coords[coords.length - 1].x.toFixed(1)},${bottom}L${coords[0].x.toFixed(1)},${bottom}Z`
      return {
        left,
        bottom,
        coords,
        line,
        area,
        end: coords[coords.length - 1],
        ticks: ticks.map((value) => ({ value, y: y(value) })),
        firstDay: formatDay(t0),
        lastDay: formatDay(t1),
      }
    })

    const hover = computed(() => {
      if (hoverIndex.value == null || !geo.value) return null
      const point = props.points[hoverIndex.value]
      const c = geo.value.coords[hoverIndex.value]
      return point && c ? { point, x: c.x, y: c.y } : null
    })

    const label = computed(() => {
      const points = props.points
      if (points.length < 2) return ''
      const last = points[points.length - 1]
      return t('priceChart.aria', { from: formatDay(points[0].t), to: formatDay(last.t), last: formatPrice(last.price) })
    })

    function onPointerMove(event) {
      const svg = event.currentTarget.ownerSVGElement
      const rect = svg?.getBoundingClientRect()
      if (!rect?.width || !geo.value) return
      const px = ((event.clientX - rect.left) / rect.width) * width.value
      let best = 0
      geo.value.coords.forEach((c, i) => {
        if (Math.abs(c.x - px) < Math.abs(geo.value.coords[best].x - px)) best = i
      })
      hoverIndex.value = best
    }
    function onKeydown(event) {
      const count = props.points.length
      if (!count) return
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        event.preventDefault()
        const current = hoverIndex.value ?? (event.key === 'ArrowLeft' ? count : -1)
        hoverIndex.value = Math.min(count - 1, Math.max(0, current + (event.key === 'ArrowRight' ? 1 : -1)))
      }
    }

    return { HEIGHT, PAD, wrapRef, width, geo, hover, hoverIndex, label, onPointerMove, onKeydown, formatPrice, formatAmount, formatDay }
  },
}
</script>

<style scoped>
.mpc { position: relative; min-width: 0; }
.mpc-svg { display: block; overflow: visible; }
.mpc-svg:focus { outline: none; }
.mpc-svg:focus-visible { outline: 2px solid var(--cream); outline-offset: 2px; border-radius: 2px; }
.mpc-grid line { stroke: var(--border); stroke-width: 1; opacity: 0.7; }
.mpc-grid text { fill: var(--muted); font-size: 10px; font-weight: 600; font-variant-numeric: tabular-nums; }
.mpc-area { fill: var(--accent); opacity: 0.12; }
.mpc-line { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.mpc-end { fill: var(--accent); stroke: var(--panel); stroke-width: 2; }
.mpc-crosshair { stroke: var(--cream); stroke-width: 1; opacity: 0.35; }
.mpc-hover-dot { fill: var(--cream); stroke: var(--panel); stroke-width: 2; }
.mpc-hit { fill: transparent; cursor: crosshair; }
.mpc-tooltip { top: 4px; min-width: 132px; transform: translateX(12px); }
.mpc-tooltip--left { transform: translateX(calc(-100% - 12px)); }
.mpc-tooltip-title { font-size: 12px; font-weight: 800; color: var(--text); margin-bottom: 3px; }
.mpc-tooltip-row { display: flex; justify-content: space-between; gap: 12px; color: var(--muted); white-space: nowrap; }
.mpc-tooltip-row span:last-child { color: var(--text); font-weight: 700; }
</style>
