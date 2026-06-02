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
          <span v-if="tab.id === 'dashboard' && pinned.length" class="tab-badge">{{ pinned.length }}</span>
        </button>
      </nav>
    </header>
    <main>
      <Dashboard v-if="activeTab === 'server'" />
      <PinnedDashboard v-else-if="activeTab === 'dashboard'" />
      <PlayerInventory v-else-if="activeTab === 'inventory'" />
      <MarketTab v-else-if="activeTab === 'market'" />
    </main>
  </div>
</template>

<script>
import { ref } from 'vue'
import Dashboard from './components/Dashboard.vue'
import PlayerInventory from './components/PlayerInventory.vue'
import PinnedDashboard from './components/PinnedDashboard.vue'
import { usePinned } from './composables/usePinned'

import MarketTab from './components/MarketTab.vue'

export default {
  components: { Dashboard, PlayerInventory, PinnedDashboard, MarketTab },
  setup() {
    const activeTab = ref('server')
    const { pinned } = usePinned()
    const tabs = [
      { id: 'server', label: 'Серверы' },
      { id: 'dashboard', label: 'Дашборд' },
      { id: 'inventory', label: 'Персонаж' },
      { id: 'market', label: 'Рынок' },
    ]
    return { activeTab, tabs, pinned }
  },
}
</script>

<style>
* { box-sizing: border-box; }
.app { font-family: Inter, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial; padding: 16px; }
.header {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-bottom: 18px;
}
.logo { font-size: 22px; font-weight: 800; color: #0f172a; letter-spacing: -0.5px; }
.tabs { display: flex; gap: 4px; }
.tab {
  padding: 6px 16px;
  border-radius: 7px;
  border: none;
  background: transparent;
  font-size: 14px;
  font-weight: 500;
  color: #64748b;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.tab:hover { background: #f1f5f9; color: #0f172a; }
.tab.active { background: #3b82f6; color: white; }
.tab-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  margin-left: 5px;
  background: #ef4444;
  color: white;
  border-radius: 9px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}
</style>
