<template>
  <div class="app">
    <header class="header">BitDash</header>
    <main>
      <Dashboard />
    </main>
  </div>
</template>

<script>
import Dashboard from './components/Dashboard.vue'
export default { components: { Dashboard } }
</script>

<style>
.app { font-family: Inter, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial; padding: 16px; }
.header { font-size: 24px; font-weight: 700; margin-bottom: 12px }
</style>
