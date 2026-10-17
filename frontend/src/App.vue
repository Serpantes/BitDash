<template>
  <div class="app">
    <header class="app-header">
      <span class="app-logo"><span class="app-logo-mark" aria-hidden="true"></span>BitDash</span>
      <nav class="app-tabs" role="tablist">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          role="tab"
          class="app-tab"
          :class="{ active: activeTab === tab.id }"
          :aria-selected="activeTab === tab.id"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </nav>
      <div id="app-actions" class="app-actions"></div>
      <div class="app-status">
        <BitjitaStatus />
        <LocaleSwitch />
      </div>
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
import LocaleSwitch from './components/LocaleSwitch.vue'
import { t } from './i18n'

export default {
  components: { Dashboard, PinnedDashboard, BitjitaStatus, LocaleSwitch },
  setup() {
    const activeTab = ref('dashboard')
    const tabs = [
      { id: 'dashboard', get label() { return t('app.tabs.dashboard') } },
      { id: 'server', get label() { return t('app.tabs.server') } },
    ]
    return { activeTab, tabs }
  },
}
</script>

<style>
.app { min-height: 100vh; padding: 0 16px 24px; }
.app-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 22px;
  margin: 0 -16px 16px;
  padding: 10px 16px;
  background: linear-gradient(180deg, rgba(31, 89, 135, 0.55), rgba(21, 19, 29, 0.92));
  border-bottom: 1px solid rgba(233, 223, 196, 0.22);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(6px);
  flex-wrap: wrap;
}
.app-logo {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  font-family: var(--font-title);
  font-size: 19px;
  font-weight: 600;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--text);
}
.app-logo-mark {
  width: 11px;
  height: 11px;
  border: 1.5px solid var(--cream);
  background: var(--accent-strong);
  transform: rotate(45deg);
}
.app-tabs { display: flex; gap: 6px; }
.app-tab {
  position: relative;
  padding: 6px 14px;
  border: none;
  border-radius: 3px;
  background: transparent;
  color: var(--text-soft);
  font-family: var(--font-title);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.app-tab:hover { color: var(--text); background: rgba(233, 223, 196, 0.08); }
.app-tab.active { background: var(--cream); color: var(--on-cream); }
.app-actions { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.app-actions:empty { display: none; }
.app-status { display: flex; align-items: center; gap: 8px; }
.app-actions:empty + .app-status { margin-left: auto; }
</style>
