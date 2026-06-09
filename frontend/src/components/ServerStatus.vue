<template>
  <div class="card server-status">
    <div class="card-header">
      <span class="card-title">Статус серверов</span>
      <span class="refresh-btn" @click="load" title="Обновить">&#x21bb;</span>
    </div>

    <div v-if="loading" class="placeholder">Загрузка...</div>
    <div v-else-if="error" class="error-msg">{{ error }}</div>
    <div v-else class="stats-row">
      <div class="stat">
        <div class="stat-value">{{ totalOnline }}</div>
        <div class="stat-label">Игроков онлайн</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ regionCount }}</div>
        <div class="stat-label">Регионов</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ inQueue }}</div>
        <div class="stat-label">В очереди</div>
      </div>
      <div class="stat">
        <div class="stat-value">
          <span class="badge" :class="serverOnline ? 'badge-ok' : 'badge-fail'">
            {{ serverOnline ? 'Online' : 'Offline' }}
          </span>
        </div>
        <div class="stat-label">Сервер</div>
      </div>
    </div>

    <div v-if="updatedAt" class="updated-at">Обновлено: {{ updatedAt }}</div>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { fetchBitjita } from '../api'

export default {
  name: 'ServerStatus',
  setup() {
    const data = ref(null)
    const loading = ref(true)
    const error = ref(null)
    const updatedAt = ref(null)
    let timer = null

    async function load() {
      try {
        error.value = null
        const res = await fetchBitjita('/status')
        data.value = res
        updatedAt.value = new Date().toLocaleTimeString('ru-RU')
      } catch (e) {
        error.value = e.message
      } finally {
        loading.value = false
      }
    }

    const totalOnline = computed(() => {
      if (!data.value) return '—'
      return data.value.totalSignedIn ?? '—'
    })

    const inQueue = computed(() => {
      if (!data.value) return '—'
      return data.value.totalInQueue ?? 0
    })

    const regionCount = computed(() => {
      if (!data.value) return '—'
      return data.value.count ?? '—'
    })

    const serverOnline = computed(() => {
      if (!data.value) return false
      return (data.value.count ?? 0) > 0
    })

    onMounted(() => {
      load()
      timer = setInterval(load, 30000)
    })

    onUnmounted(() => clearInterval(timer))

    return { loading, error, updatedAt, totalOnline, inQueue, regionCount, serverOnline, load }
  }
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
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
.stats-row {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}
.stat { flex: 1; min-width: 80px; }
.stat-value { font-size: 28px; font-weight: 700; color: var(--text); }
.stat-label { font-size: 12px; color: var(--muted); margin-top: 2px; }
.badge {
  display: inline-block;
  font-size: 13px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 12px;
}
.badge-ok { background: rgba(117, 224, 167, 0.16); color: var(--success); }
.badge-fail { background: rgba(255, 138, 154, 0.16); color: var(--danger); }
.placeholder, .error-msg { font-size: 14px; color: var(--muted); padding: 8px 0; }
.error-msg { color: var(--danger); }
.updated-at { font-size: 11px; color: var(--muted); margin-top: 12px; }
</style>
