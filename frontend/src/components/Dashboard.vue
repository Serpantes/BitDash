<template>
  <div class="servers">
    <ServerStatus />
    <RegionTable />
  </div>
</template>

<script>
import ServerStatus from './ServerStatus.vue'
import RegionTable from './RegionTable.vue'

export default {
  components: { ServerStatus, RegionTable },
}
</script>

<style scoped>
.servers {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 420px);
  gap: 14px;
  align-items: start;
}
@media (max-width: 900px) {
  .servers { grid-template-columns: 1fr; }
}
</style>
