<template>
  <div class="app">
    <header class="header">
      <span class="logo">BitDash</span>
      <nav class="tabs">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          class="tab"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </nav>
      <BitjitaStatus />
    </header>
    <main>
      <PinnedDashboard v-if="activeTab === 'dashboard'" />
      <Dashboard v-else-if="activeTab === 'server'" />
    </main>
  </div>
</template>

<script>
import { ref } from 'vue'
import Dashboard from './components/Dashboard.vue'
import PinnedDashboard from './components/PinnedDashboard.vue'
import BitjitaStatus from './components/BitjitaStatus.vue'

export default {
  components: { Dashboard, PinnedDashboard, BitjitaStatus },
  setup() {
    const activeTab = ref('dashboard')
    const tabs = [
      { id: 'dashboard', label: 'Дашборд' },
      { id: 'server', label: 'Серверы' },
    ]
    return { activeTab, tabs }
  },
}
</script>

<style>
:root {
  --app-bg: #171625;
  --app-bg-soft: #252437;
  --surface: #252437;
  --surface-raised: #2f2e43;
  --surface-hover: #35344a;
  --border: #45435c;
  --border-strong: #6a687e;
  --text: #f4f2ff;
  --text-soft: #d7d3ea;
  --muted: #a7a1bd;
  --muted-strong: #c4bfd7;
  --accent: #8aa4ff;
  --accent-strong: #6f8eff;
  --danger: #ff8a9a;
  --success: #75e0a7;
  --warning: #f8c66d;
  color-scheme: dark;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background:
    radial-gradient(circle at top left, rgba(111, 142, 255, 0.14), transparent 34rem),
    linear-gradient(180deg, #171625 0%, #201f31 100%);
  color: var(--text);
}
.app { min-height: 100vh; font-family: Inter, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial; padding: 16px; }
.header {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}
.logo { font-size: 22px; font-weight: 800; color: var(--text); letter-spacing: -0.5px; }
.tabs { display: flex; gap: 4px; }
.tab {
  padding: 6px 16px;
  border-radius: 7px;
  border: none;
  background: transparent;
  font-size: 14px;
  font-weight: 500;
  color: var(--muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.tab:hover { background: var(--surface-hover); color: var(--text); }
.tab.active { background: var(--accent-strong); color: white; }
</style>
