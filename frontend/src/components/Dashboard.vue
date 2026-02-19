<template>
  <div class="dashboard">
    <RequestStatus :endpoints="['/api/health', '/api/data']" />
    <div class="card">
      <h3>External Data</h3>
      <canvas id="chart" width="400" height="160"></canvas>
    </div>
  </div>
</template>

<script>
import { onMounted, ref } from 'vue'
import RequestStatus from './RequestStatus.vue'

export default {
  components: { RequestStatus },
  setup() {
    const data = ref([])

    async function load() {
      try {
        const res = await fetch('/api/data')
        const json = await res.json()
        // normalize to array of numbers for demo
        if (Array.isArray(json)) data.value = json
        else if (json && json.values) data.value = json.values
        else data.value = [Math.random()*10, Math.random()*10, Math.random()*10]
        drawChart()
      } catch (e) {
        console.error(e)
      }
    }

    function drawChart() {
      const ctx = document.getElementById('chart')
      if (!ctx || !window.Chart) return
      const labels = data.value.map((_,i)=>`#${i+1}`)
      new window.Chart(ctx.getContext('2d'), {
        type: 'line',
        data: {
          labels,
          datasets: [{
            label: 'Values',
            data: data.value,
            borderColor: '#3b82f6',
            backgroundColor: 'rgba(59,130,246,0.2)'
          }]
        },
        options: { responsive: true, maintainAspectRatio: false }
      })
    }

    onMounted(() => { load() })
    return {}
  }
}
</script>

<style>
.dashboard { display:flex; gap:16px }
.card { background: white; border-radius:8px; padding:16px; box-shadow:0 6px 20px rgba(2,6,23,0.08); width:100%; height:300px }
</style>
