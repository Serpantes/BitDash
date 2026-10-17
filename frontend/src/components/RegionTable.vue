<template>
  <section class="bc-panel">
    <header class="bc-panel-head">
      <h2 class="bc-title">{{ t('server.regions') }}</h2>
      <span class="bc-ornament"></span>
    </header>
    <div class="bc-panel-body">
      <div v-if="loading" class="bc-muted-note">{{ t('common.loading') }}</div>
      <div v-else-if="error" class="bc-error">{{ error }}</div>
      <div v-else-if="regions.length === 0" class="bc-muted-note">{{ t('common.noDataCap') }}</div>
      <ul v-else class="regions">
        <li
          v-for="r in sorted"
          :key="r.regionId ?? r.regionName"
          class="region"
          :class="{ 'region--off': r.active === false }"
        >
          <span class="region-name">{{ r.regionName ?? '—' }}<span v-if="r.regionId" class="region-id">R{{ r.regionId }}</span></span>
          <span class="region-bar" aria-hidden="true">
            <span class="region-fill" :style="{ width: `${barPct(r)}%` }"></span>
          </span>
          <span class="region-count">
            <template v-if="r.active === false">{{ t('common.offline') }}</template>
            <template v-else>
              {{ r.signedInPlayers ?? 0 }}<span v-if="r.playersInQueue > 0" class="region-queue" :title="t('server.queueTitle')"> +{{ r.playersInQueue }}</span>
            </template>
          </span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { fetchBitjita } from '../api'

export default {
  name: 'RegionTable',
  setup() {
    const rawData = ref(null)
    const loading = ref(true)
    const error = ref(null)
    let timer = null

    async function load() {
      try {
        error.value = null
        rawData.value = await fetchBitjita('/regions/status')
      } catch (e) {
        error.value = e.message
      } finally {
        loading.value = false
      }
    }

    const regions = computed(() => {
      if (!rawData.value) return []
      if (Array.isArray(rawData.value)) return rawData.value
      if (Array.isArray(rawData.value.regions)) return rawData.value.regions
      if (Array.isArray(rawData.value.data)) return rawData.value.data
      return []
    })

    const sorted = computed(() =>
      [...regions.value].sort((a, b) => (b.signedInPlayers ?? 0) - (a.signedInPlayers ?? 0))
    )
    const maxPlayers = computed(() => Math.max(1, ...regions.value.map((r) => r.signedInPlayers ?? 0)))

    function barPct(region) {
      return ((region.signedInPlayers ?? 0) / maxPlayers.value) * 100
    }

    onMounted(() => {
      load()
      timer = setInterval(load, 30000)
    })

    onUnmounted(() => clearInterval(timer))

    return { loading, error, regions, sorted, barPct }
  },
}
</script>

<style scoped>
.regions { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.region {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 90px 56px;
  align-items: center;
  gap: 12px;
  min-height: 32px;
  border-bottom: 1px solid rgba(233, 223, 196, 0.07);
  font-size: 13px;
}
.region:last-child { border-bottom: none; }
.region-name { min-width: 0; color: var(--text); font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.region-id { margin-left: 7px; font-size: 11px; font-weight: 600; color: var(--muted); }
.region-bar { height: 6px; border-radius: 3px; background: rgba(20, 18, 30, 0.5); overflow: hidden; }
.region-fill { display: block; height: 100%; border-radius: 3px; background: var(--accent); }
.region-count { text-align: right; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.region-queue { color: var(--warning); }
.region--off .region-name, .region--off .region-count { color: var(--muted); font-weight: 600; }
</style>
