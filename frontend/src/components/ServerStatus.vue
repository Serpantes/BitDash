<template>
  <section class="bc-panel">
    <header class="bc-panel-head">
      <h2 class="bc-title">{{ t('server.online') }}</h2>
      <span class="bc-ornament"></span>
    </header>
    <div class="bc-panel-body online-body">
      <div v-if="loading" class="bc-muted-note">{{ t('common.loading') }}</div>
      <div v-else-if="error" class="bc-error">{{ error }}</div>
      <div v-else class="online-stats">
        <div class="online-stat">
          <span class="online-value">{{ totalOnline }}</span>
          <span class="online-label">{{ t('server.signedIn') }}</span>
        </div>
        <div class="online-stat" :class="{ 'online-stat--queue': inQueue > 0 }">
          <span class="online-value online-value--small">{{ inQueue }}</span>
          <span class="online-label">{{ t('server.inQueue') }}</span>
        </div>
      </div>
      <h3 class="bc-section">{{ t('server.last24h') }}</h3>
      <PopulationChart />
    </div>
  </section>
</template>

<script>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { fetchBitjita } from '../api'
import { formatNumber } from '../i18n'
import PopulationChart from './PopulationChart.vue'

export default {
  name: 'ServerStatus',
  components: { PopulationChart },
  setup() {
    const data = ref(null)
    const loading = ref(true)
    const error = ref(null)
    let timer = null

    async function load() {
      try {
        error.value = null
        data.value = await fetchBitjita('/status')
      } catch (e) {
        error.value = e.message
      } finally {
        loading.value = false
      }
    }

    const totalOnline = computed(() => formatNumber(data.value?.totalSignedIn ?? 0))
    const inQueue = computed(() => data.value?.totalInQueue ?? 0)

    onMounted(() => {
      load()
      timer = setInterval(load, 30000)
    })

    onUnmounted(() => clearInterval(timer))

    return { loading, error, totalOnline, inQueue }
  },
}
</script>

<style scoped>
.online-body { gap: 14px; }
.online-stats { display: flex; align-items: flex-end; gap: 36px; }
.online-stat { display: flex; align-items: baseline; gap: 8px; }
.online-value { font-size: 44px; font-weight: 800; line-height: 1; color: var(--text); font-variant-numeric: tabular-nums; }
.online-value--small { font-size: 24px; color: var(--text-soft); }
.online-label { font-size: 13px; color: var(--muted); }
.online-stat--queue .online-value { color: var(--warning); }
</style>
