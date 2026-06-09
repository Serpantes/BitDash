<template>
  <div class="card population-chart">
    <div class="card-header">
      <span class="card-title">Онлайн за 24 часа</span>
      <span class="refresh-btn" @click="load" title="Обновить">&#x21bb;</span>
    </div>

    <div v-if="loading" class="placeholder">Загрузка...</div>
    <div v-else-if="error" class="error-msg">{{ error }}</div>
    <div v-else class="chart-wrap">
      <canvas ref="canvas"></canvas>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { fetchBitjita } from '../api'

export default {
  name: 'PopulationChart',
  setup() {
    const canvas = ref(null)
    const loading = ref(true)
    const error = ref(null)
    const chartData = ref(null)
    let chartInstance = null
    let timer = null

    async function load() {
      try {
        error.value = null
        const to = new Date()
        const from = new Date(to.getTime() - 24 * 60 * 60 * 1000)
        const res = await fetchBitjita('/status/chart', {
          from: from.toISOString(),
          to: to.toISOString(),
        })
        chartData.value = res
      } catch (e) {
        error.value = e.message
      } finally {
        loading.value = false
      }
    }

    function parseData(res) {
      // Real response: { count, buckets: [{ bucket (ISO string), totalSignedIn, ... }] }
      if (!res) return { labels: [], values: [] }
      const buckets = Array.isArray(res.buckets) ? res.buckets : (Array.isArray(res) ? res : [])
      if (buckets.length === 0) return { labels: [], values: [] }
      const labels = buckets.map((d) => {
        const ts = d.bucket ?? d.time ?? d.timestamp
        if (!ts) return ''
        return new Date(ts).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
      })
      const values = buckets.map((d) => d.totalSignedIn ?? d.count ?? d.online ?? 0)
      return { labels, values }
    }

    function renderChart() {
      if (!canvas.value || !window.Chart || !chartData.value) return

      const { labels, values } = parseData(chartData.value)
      if (values.length === 0) return
      const css = getComputedStyle(document.documentElement)
      const accent = css.getPropertyValue('--accent').trim() || '#8aa4ff'
      const textSoft = css.getPropertyValue('--text-soft').trim() || '#d7d3ea'
      const muted = css.getPropertyValue('--muted').trim() || '#a7a1bd'
      const border = css.getPropertyValue('--border').trim() || '#45435c'

      if (chartInstance) {
        chartInstance.destroy()
        chartInstance = null
      }

      chartInstance = new window.Chart(canvas.value.getContext('2d'), {
        type: 'line',
        data: {
          labels,
          datasets: [
            {
              label: 'Игроков онлайн',
              data: values,
              borderColor: accent,
              backgroundColor: 'rgba(138, 164, 255, 0.15)',
              borderWidth: 2,
              pointRadius: 2,
              tension: 0.3,
              fill: true,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { display: false },
            tooltip: {
              titleColor: textSoft,
              bodyColor: textSoft,
              backgroundColor: '#252437',
              borderColor: border,
              borderWidth: 1,
              callbacks: {
                label: (ctx) => ` ${ctx.parsed.y} игроков`,
              },
            },
          },
          scales: {
            x: {
              ticks: { maxTicksLimit: 8, font: { size: 11 }, color: muted },
              grid: { display: false, color: border },
            },
            y: {
              beginAtZero: false,
              ticks: { font: { size: 11 }, color: muted },
              grid: { color: border },
            },
          },
        },
      })
    }

    watch(chartData, async () => {
      await nextTick()
      renderChart()
    })

    onMounted(() => {
      load()
      timer = setInterval(load, 60000)
    })

    onUnmounted(() => {
      clearInterval(timer)
      if (chartInstance) chartInstance.destroy()
    })

    return { canvas, loading, error, load }
  },
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.card-title { font-size: 15px; font-weight: 600; color: var(--text); }
.refresh-btn {
  font-size: 18px;
  cursor: pointer;
  color: var(--muted);
  user-select: none;
  transition: color 0.15s;
}
.refresh-btn:hover { color: var(--accent); }
.chart-wrap { position: relative; height: 220px; }
.chart-wrap canvas { width: 100% !important; height: 100% !important; }
.placeholder, .error-msg { font-size: 14px; color: var(--muted); padding: 8px 0; }
.error-msg { color: var(--danger); }
</style>
