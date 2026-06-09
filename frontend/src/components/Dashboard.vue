<template>
  <div class="dashboard">
    <div class="row">
      <ServerStatus class="flex-1" />
      <RegionTable class="flex-2" />
    </div>
    <div class="row">
      <PopulationChart class="flex-1" />
    </div>
  </div>
</template>

<script>
import ServerStatus from './ServerStatus.vue'
import RegionTable from './RegionTable.vue'
import PopulationChart from './PopulationChart.vue'

export default {
  components: { ServerStatus, RegionTable, PopulationChart },
}
</script>

<style>
.dashboard { display: flex; flex-direction: column; gap: 16px; }
.row { display: flex; gap: 16px; flex-wrap: wrap; }
.flex-1 { flex: 1; min-width: 240px; }
.flex-2 { flex: 2; min-width: 320px; }
.card {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 18px 20px;
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.24);
}
</style>
