<template>
  <div class="request-status card">
    <h3>Backend requests</h3>
    <div class="controls">
      <button @click="refresh">Refresh</button>
      <label><input type="checkbox" v-model="autoRefresh"> Auto-refresh</label>
    </div>
    <ul>
      <li v-for="r in results" :key="r.url" :class="{ ok: r.ok, fail: !r.ok }">
        <div style="flex:1">
          <strong>{{ r.url }}</strong>
          <div style="font-size:12px; color:var(--muted,#555)">
            <span v-if="r.ok">OK</span>
            <span v-else>FAIL</span>
            <span v-if="r.time"> — {{ r.time }} ms</span>
            <span v-if="r.status"> — status: {{ r.status }}</span>
            <span v-if="r.error"> — error: {{ r.error }}</span>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script>
import { ref, onMounted, onUnmounted } from 'vue'
import { ping } from '../api'

export default {
  props: { endpoints: { type: Array, default: () => ['/api/health', '/api/data'] } },
  setup(props) {
    const results = ref([])
    const autoRefresh = ref(true)
    let timer = null

    async function refresh() {
      const res = await Promise.all(props.endpoints.map((e) => ping(e)))
      results.value = res
    }

    onMounted(() => {
      refresh()
      timer = setInterval(() => { if (autoRefresh.value) refresh() }, 10000)
    })

    onUnmounted(() => { clearInterval(timer) })

    return { results, autoRefresh, refresh }
  }
}
</script>

<style scoped>
.request-status ul { list-style:none; padding:0; margin:8px 0 0 0 }
.request-status li { padding:8px; border-radius:6px; margin-bottom:6px; display:flex; gap:8px; align-items:center }
.request-status li.ok { background: #ecfdf5; color:#064e3b }
.request-status li.fail { background:#fff1f2; color:#7f1d1d }
.controls { display:flex; gap:12px; align-items:center; margin-top:8px }
button { padding:6px 10px; border-radius:6px; border:1px solid #ccc; background:#f9fafb }
</style>
