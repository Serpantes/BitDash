<template>
  <div class="population-chart">
    <div v-if="loading" class="bc-muted-note">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="bc-error">{{ error }}</div>
    <div v-else class="chart-wrap">
      <canvas ref="canvas"></canvas>
    </div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { fetchBitjita } from '../api'
import { intlLocale, locale, t } from '../i18n'

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
      if (!res) return { labels: [], values: [] }
      const buckets = Array.isArray(res.buckets) ? res.buckets : (Array.isArray(res) ? res : [])
      if (buckets.length === 0) return { labels: [], values: [] }
      const labels = buckets.map((d) => {
        const ts = d.bucket ?? d.time ?? d.timestamp
        if (!ts) return ''
        return new Date(ts).toLocaleTimeString(intlLocale(), { hour: '2-digit', minute: '2-digit' })
      })
      const values = buckets.map((d) => d.totalSignedIn ?? d.count ?? d.online ?? 0)
      return { labels, values }
    }

    function renderChart() {
      if (!canvas.value || !window.Chart || !chartData.value) return

      const { labels, values } = parseData(chartData.value)
      if (values.length === 0) return
      const css = getComputedStyle(document.documentElement)
      const token = (name, fallback) => css.getPropertyValue(name).trim() || fallback
      const accent = token('--accent', '#5ea6dc')
      const text = token('--text', '#f2ebd9')
      const muted = token('--muted', '#a39d98')
      const border = token('--border', '#45424f')
      const cream = token('--cream', '#e9dfc4')
      const font = { family: token('--font-body', 'Nunito, sans-serif'), size: 11, weight: 600 }

      if (chartInstance) {
        chartInstance.destroy()
        chartInstance = null
      }

      const ctx = canvas.value.getContext('2d')
      const fill = ctx.createLinearGradient(0, 0, 0, canvas.value.clientHeight || 240)
      fill.addColorStop(0, 'rgba(94, 166, 220, 0.28)')
      fill.addColorStop(1, 'rgba(94, 166, 220, 0)')

      chartInstance = new window.Chart(ctx, {
        type: 'line',
        data: {
          labels,
          datasets: [
            {
              label: t('server.playersOnline'),
              data: values,
              borderColor: accent,
              backgroundColor: fill,
              borderWidth: 2,
              pointRadius: 0,
              pointHoverRadius: 4,
              pointHoverBackgroundColor: cream,
              pointHoverBorderColor: '#2a2737',
              pointHoverBorderWidth: 2,
              tension: 0.35,
              fill: true,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          interaction: { mode: 'index', intersect: false },
          plugins: {
            legend: { display: false },
            tooltip: {
              displayColors: false,
              titleColor: muted,
              bodyColor: text,
              titleFont: font,
              bodyFont: { ...font, size: 13, weight: 800 },
              backgroundColor: 'rgba(24, 22, 34, 0.96)',
              borderColor: 'rgba(233, 223, 196, 0.55)',
              borderWidth: 1,
              padding: 8,
              callbacks: {
                label: (item) => t('server.playersCount', { n: item.parsed.y }),
              },
            },
          },
          scales: {
            x: {
              ticks: { maxTicksLimit: 8, font, color: muted, maxRotation: 0 },
              grid: { display: false },
              border: { color: border },
            },
            y: {
              beginAtZero: false,
              ticks: { font, color: muted, maxTicksLimit: 5 },
              grid: { color: border },
              border: { display: false },
            },
          },
        },
      })
    }

    watch([chartData, locale], async () => {
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
.chart-wrap { position: relative; height: 260px; }
.chart-wrap canvas { width: 100% !important; height: 100% !important; }
</style>
