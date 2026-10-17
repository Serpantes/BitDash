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
    <span>BitJita</span>
    <span class="bitjita-status-value">{{ valueLabel }}</span>
  </button>
</template>

<script>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { pingBitjita } from '../api'
import { t } from '../i18n'

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
      if (checking.value && latency.value == null) return 'bitjita-status--checking'
      if (!ok.value) return 'bitjita-status--down'
      if (latency.value != null && latency.value >= SLOW_MS) return 'bitjita-status--slow'
      return 'bitjita-status--ok'
    })

    const valueLabel = computed(() => {
      if (checking.value && latency.value == null) return '…'
      if (!ok.value) return t('status.down')
      return `${latency.value} ms`
    })

    const tooltip = computed(() => {
      const parts = ['BitJita API']
      if (latency.value != null) parts.push(t('status.response', { ms: latency.value }))
      if (statusCode.value != null) parts.push(`HTTP ${statusCode.value}`)
      if (error.value) parts.push(t('status.error', { error: error.value }))
      parts.push(t('status.clickToCheck'))
      return parts.join(' · ')
    })

    onMounted(() => {
      check()
      timer = setInterval(check, INTERVAL_MS)
    })

    onUnmounted(() => clearInterval(timer))

    return { checking, valueLabel, statusClass, tooltip, check }
  },
}
</script>

<style scoped>
.bitjita-status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 10px;
  border: 1px solid var(--frame-soft);
  border-radius: 3px;
  background: rgba(20, 18, 30, 0.35);
  color: var(--text-soft);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  cursor: pointer;
}
.bitjita-status:hover:not(:disabled) { border-color: var(--frame); color: var(--text); }
.bitjita-status:disabled { cursor: default; }
.bitjita-status-dot {
  width: 7px;
  height: 7px;
  flex-shrink: 0;
  transform: rotate(45deg);
  background: var(--muted);
}
.bitjita-status-value { font-variant-numeric: tabular-nums; color: var(--muted); font-weight: 600; }
.bitjita-status--checking .bitjita-status-dot { animation: bitjita-pulse 1.2s ease-in-out infinite; }
.bitjita-status--ok .bitjita-status-dot { background: var(--success); }
.bitjita-status--slow .bitjita-status-dot { background: var(--warning); }
.bitjita-status--slow .bitjita-status-value { color: var(--warning); }
.bitjita-status--down .bitjita-status-dot { background: var(--danger); }
.bitjita-status--down .bitjita-status-value { color: var(--danger); }
@keyframes bitjita-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}
</style>
