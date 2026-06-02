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
              borderColor: '#3b82f6',
              backgroundColor: 'rgba(59,130,246,0.15)',
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
              callbacks: {
                label: (ctx) => ` ${ctx.parsed.y} игроков`,
              },
            },
          },
          scales: {
            x: {
              ticks: { maxTicksLimit: 8, font: { size: 11 } },
              grid: { display: false },
            },
            y: {
              beginAtZero: false,
              ticks: { font: { size: 11 } },
              grid: { color: '#f1f5f9' },
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
.card-title { font-size: 15px; font-weight: 600; color: #1e293b; }
.refresh-btn {
  font-size: 18px;
  cursor: pointer;
  color: #64748b;
  user-select: none;
  transition: color 0.15s;
}
.refresh-btn:hover { color: #3b82f6; }
.chart-wrap { position: relative; height: 220px; }
.chart-wrap canvas { width: 100% !important; height: 100% !important; }
.placeholder, .error-msg { font-size: 14px; color: #64748b; padding: 8px 0; }
.error-msg { color: #b91c1c; }
</style>
