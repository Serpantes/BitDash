<template>
  <div class="card region-table">
    <div class="card-header">
      <span class="card-title">Регионы</span>
      <span class="refresh-btn" @click="load" title="Обновить">&#x21bb;</span>
    </div>

    <div v-if="loading" class="placeholder">Загрузка...</div>
    <div v-else-if="error" class="error-msg">{{ error }}</div>
    <div v-else-if="regions.length === 0" class="placeholder">Нет данных</div>
    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Регион</th>
            <th>Игроков</th>
            <th>Очередь</th>
            <th>Статус</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in sorted" :key="r.regionId ?? r.regionName">
            <td class="region-name">{{ r.regionName ?? '—' }}<span v-if="r.regionId" class="region-id"> (R{{ r.regionId }})</span></td>
            <td class="num">{{ r.signedInPlayers ?? '—' }}</td>
            <td class="num">{{ r.playersInQueue ?? 0 }}</td>
            <td>
              <span class="badge" :class="r.active ? 'badge-ok' : 'badge-fail'">
                {{ r.active ? 'Online' : 'Offline' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="updatedAt" class="updated-at">Обновлено: {{ updatedAt }}</div>
  </div>
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
    const updatedAt = ref(null)
    let timer = null

    async function load() {
      try {
        error.value = null
        const res = await fetchBitjita('/regions/status')
        rawData.value = res
        updatedAt.value = new Date().toLocaleTimeString('ru-RU')
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

    onMounted(() => {
      load()
      timer = setInterval(load, 30000)
    })

    onUnmounted(() => clearInterval(timer))

    return { loading, error, regions, sorted, updatedAt, load }
  }
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
.table-wrap { overflow-x: auto; }
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
th {
  text-align: left;
  padding: 6px 10px;
  color: var(--muted);
  font-weight: 500;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(106, 104, 126, 0.32);
  color: var(--text-soft);
}
tr:last-child td { border-bottom: none; }
.region-name { font-weight: 500; color: var(--text); }
.region-id { font-weight: 400; color: var(--muted); font-size: 12px; }
.num { font-variant-numeric: tabular-nums; }
.badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
}
.badge-ok { background: rgba(117, 224, 167, 0.16); color: var(--success); }
.badge-fail { background: rgba(255, 138, 154, 0.16); color: var(--danger); }
.placeholder, .error-msg { font-size: 14px; color: var(--muted); padding: 8px 0; }
.error-msg { color: var(--danger); }
.updated-at { font-size: 11px; color: var(--muted); margin-top: 10px; }
</style>
