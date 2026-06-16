<template>
  <button
    type="button"
    class="bitjita-status"
    :class="statusClass"
    :title="tooltip"
    :disabled="checking"
    @click="check"
  >
    <span class="bitjita-status-dot" />
    <span class="bitjita-status-label">{{ label }}</span>
    <span v-if="latency != null" class="bitjita-status-ms">{{ latency }} ms</span>
  </button>
</template>

<script>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { pingBitjita } from '../api'

const SLOW_MS = 3000
const INTERVAL_MS = 30000
const TIMEOUT_MS = 10000

export default {
  name: 'BitjitaStatus',
  setup() {
    const checking = ref(true)
    const ok = ref(false)
    const latency = ref(null)
    const statusCode = ref(null)
    const error = ref(null)
    let timer = null

    async function check() {
      checking.value = true
      const result = await pingBitjita('/status', TIMEOUT_MS)
      latency.value = result.time
      statusCode.value = result.status
      error.value = result.error ?? null
      ok.value = result.ok
      checking.value = false
    }

    const statusClass = computed(() => {
      if (checking.value) return 'bitjita-status--checking'
      if (!ok.value) return 'bitjita-status--down'
      if (latency.value != null && latency.value >= SLOW_MS) return 'bitjita-status--slow'
      return 'bitjita-status--ok'
    })

    const label = computed(() => {
      if (checking.value) return 'BitJita · проверка…'
      if (!ok.value) return 'BitJita · недоступен'
      if (latency.value != null && latency.value >= SLOW_MS) return 'BitJita · медленно'
      return 'BitJita · доступен'
    })

    const tooltip = computed(() => {
      if (checking.value) return 'Проверка доступности BitJita API…'
      const parts = ['BitJita API (/api/status)']
      if (latency.value != null) parts.push(`время ответа: ${latency.value} ms`)
      if (statusCode.value != null) parts.push(`HTTP ${statusCode.value}`)
      if (error.value) parts.push(`ошибка: ${error.value}`)
      parts.push('Нажмите для повторной проверки')
      return parts.join(' · ')
    })

    onMounted(() => {
      check()
      timer = setInterval(check, INTERVAL_MS)
    })

    onUnmounted(() => clearInterval(timer))

    return { checking, latency, label, statusClass, tooltip, check }
  },
}
</script>

<style scoped>
.bitjita-status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
}
.bitjita-status:hover:not(:disabled) {
  background: var(--surface-hover);
  color: var(--text);
}
.bitjita-status:disabled { cursor: default; }
.bitjita-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--muted);
}
.bitjita-status--checking .bitjita-status-dot {
  background: var(--muted);
  animation: bitjita-pulse 1.2s ease-in-out infinite;
}
.bitjita-status--ok {
  border-color: rgba(117, 224, 167, 0.35);
  color: var(--success);
}
.bitjita-status--ok .bitjita-status-dot { background: var(--success); }
.bitjita-status--slow {
  border-color: rgba(248, 198, 109, 0.35);
  color: var(--warning);
}
.bitjita-status--slow .bitjita-status-dot { background: var(--warning); }
.bitjita-status--down {
  border-color: rgba(255, 138, 154, 0.35);
  color: var(--danger);
}
.bitjita-status--down .bitjita-status-dot { background: var(--danger); }
.bitjita-status-label { white-space: nowrap; }
.bitjita-status-ms {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  opacity: 0.85;
}
@keyframes bitjita-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.45; transform: scale(0.85); }
}
</style>
