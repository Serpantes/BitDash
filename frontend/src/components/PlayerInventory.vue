<template>
  <div class="player-inventory">
    <div class="card search-card">
      <form @submit.prevent="searchPlayer" class="search-form">
        <input v-model="query" type="text" :placeholder="t('inventoryPage.searchPlaceholder')" class="search-input" :disabled="searching" />
        <button type="submit" :disabled="searching || !query.trim()" class="search-btn">{{ searching ? t('common.searching') : t('inventoryPage.find') }}</button>
      </form>
      <ul v-if="results.length > 0 && !selectedPlayer" class="results-list">
        <li v-for="p in results" :key="p.entityId" @click="selectPlayer(p)" class="result-item">
          <span class="player-name">{{ p.username }}</span>
          <span class="player-meta">{{ p.signedIn ? t('common.online') : t('common.offline') }}</span>
        </li>
      </ul>
      <div v-if="searchError" class="error-msg">{{ searchError }}</div>
    </div>

    <template v-if="selectedPlayer">
      <div class="player-header card">
        <div>
          <span class="player-title">{{ selectedPlayer.username }}</span>
          <span class="player-online" :class="selectedPlayer.signedIn ? 'online' : 'offline'">
            {{ selectedPlayer.signedIn ? t('common.online') : t('common.offline') }}
          </span>
        </div>
        <button class="reset-btn" @click="reset">✕ {{ t('inventoryPage.change') }}</button>
      </div>

      <div v-if="invLoading" class="card placeholder">{{ t('wizard.loadingInventories') }}</div>
      <div v-else-if="invError" class="card error-msg">{{ invError }}</div>
      <template v-else>
        <div v-for="inv in inventories" :key="inv.entityId" class="card inv-card">
          <div class="inv-header">
            <span class="inv-name">{{ inv.inventoryName }}</span>
            <span v-if="inv._isHousing" class="inv-house">{{ t('inventoryPage.house') }}</span>
            <span v-if="inv.regionId" class="inv-region">R{{ inv.regionId }}</span>
            <span v-if="inv.claimName" class="inv-claim">{{ inv.claimName }}</span>
            <button
              class="pin-btn"
              :class="{ pinned: isPinned(inv.entityId) }"
              @click="togglePin(inv)"
              :title="isPinned(inv.entityId) ? t('dashboard.unpin') : t('inventoryPage.pinTitle')"
            >{{ isPinned(inv.entityId) ? t('inventoryPage.pinned') : t('inventoryPage.pin') }}</button>
          </div>

          <div v-if="!hasInventoryGrid(inv)" class="empty-inv">{{ t('common.empty') }}</div>
          <InventoryGrid v-else :sections="sectionsFor(inv)" split :fill="false" />
        </div>
        <div v-if="inventories.length === 0" class="card placeholder">{{ t('inventoryPage.noInventories') }}</div>
      </template>
    </template>
  </div>
</template>

<script>
import { onMounted, ref } from 'vue'
import { fetchBitjita } from '../api'
import { t } from '../i18n'
import { ensureStorageCapacities, inventorySections } from '../inventoryLayout'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'
import InventoryGrid from './InventoryGrid.vue'

export default {
  name: 'PlayerInventory',
  components: { InventoryGrid },
  setup() {
    const query = ref('')
    const searching = ref(false)
    const searchError = ref(null)
    const results = ref([])
    const selectedPlayer = ref(null)
    const inventories = ref([])
    const invLoading = ref(false)
    const invError = ref(null)
    const { isPinned, pin, unpin } = usePinned()
    const { prefetch, seedCatalog } = useItemCache()
    onMounted(ensureStorageCapacities)

    async function searchPlayer() {
      const q = query.value.trim()
      if (!q) return
      searching.value = true
      searchError.value = null
      results.value = []
      selectedPlayer.value = null
      try {
        const res = await fetchBitjita('/players', { q })
        results.value = res.players ?? []
        if (results.value.length === 0) searchError.value = t('wizard.playerNotFound')
      } catch (e) {
        searchError.value = e.message
      } finally {
        searching.value = false
      }
    }

    async function selectPlayer(player) {
      selectedPlayer.value = player
      results.value = []
      invLoading.value = true
      invError.value = null
      inventories.value = []
      try {
        const [invRes, housingRes] = await Promise.all([
          fetchBitjita(`/players/${player.entityId}/inventories`),
          fetchBitjita(`/players/${player.entityId}/housing`).catch(() => null),
        ])
        const invs = invRes.inventories ?? []
        seedCatalog(invRes)
        const houses = Array.isArray(housingRes) ? housingRes.filter((h) => !h.isEmpty) : []
        const houseInvs = await Promise.all(houses.map((h) =>
          fetchBitjita(`/players/${player.entityId}/housing/${h.buildingEntityId}`)
            .then((detail) => {
              seedCatalog(detail)
              return (detail.inventories ?? []).map((hi) => ({
                entityId: hi.entityId,
                inventoryName: hi.buildingNickname || hi.buildingName || detail.buildingName || 'House',
                regionId: detail.regionId ?? null,
                claimName: detail.claimName ?? null,
                buildingName: hi.buildingName ?? null,
                houseName: detail.buildingName ?? null,
                pockets: (hi.inventory ?? []).map((p) => ({
                  ...p,
                  contents: p.contents ? {
                    itemId: p.contents.item_id ?? p.contents.itemId,
                    itemType: p.contents.item_type === 'cargo' ? 1 : 0,
                    quantity: p.contents.quantity,
                  } : null,
                })),
                _isHousing: true,
              }))
            })
            .catch(() => [])
        ))
        const allInvs = [...invs, ...houseInvs.flat()]
        inventories.value = allInvs
        await prefetch(allInvs)
      } catch (e) {
        invError.value = e.message
      } finally {
        invLoading.value = false
      }
    }

    function reset() {
      selectedPlayer.value = null
      results.value = []
      inventories.value = []
      query.value = ''
    }
    function sectionsFor(inv) { return inventorySections(inv.pockets, inv) }
    function hasInventoryGrid(inv) {
      const sections = sectionsFor(inv)
      return sections.items.length > 0 || sections.cargo.length > 0
    }
    function togglePin(inv) { isPinned(inv.entityId) ? unpin(inv.entityId) : pin(selectedPlayer.value, inv) }

    return {
      query, searching, searchError, results, selectedPlayer, inventories, invLoading, invError,
      searchPlayer, selectPlayer, reset, sectionsFor, hasInventoryGrid, isPinned, togglePin,
    }
  },
}
</script>

<style scoped>
.player-inventory { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.search-card { padding: 14px 18px; }
.search-form { display: flex; gap: 8px; }
.search-input { flex: 1; padding: 8px 12px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); color: var(--text); font-size: 14px; outline: none; }
.search-input:focus { border-color: var(--accent); }
.search-btn { padding: 8px 18px; background: var(--accent-strong); color: white; border: none; border-radius: 7px; font-size: 14px; font-weight: 500; cursor: pointer; }
.results-list { list-style: none; margin: 8px 0 0; padding: 0; border: 1px solid var(--border); border-radius: 7px; overflow: hidden; background: var(--surface-raised); }
.result-item { display: flex; justify-content: space-between; align-items: center; padding: 9px 14px; cursor: pointer; font-size: 14px; }
.result-item:not(:last-child) { border-bottom: 1px solid var(--border); }
.result-item:hover { background: var(--surface-hover); }
.player-name, .player-title, .inv-name { font-weight: 600; color: var(--text); }
.player-meta, .empty-inv, .placeholder { color: var(--muted); }
.player-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 18px; }
.player-title { font-size: 17px; margin-right: 10px; }
.player-online { font-size: 12px; font-weight: 500; padding: 2px 8px; border-radius: 10px; }
.player-online.online { background: rgba(117, 224, 167, 0.18); color: var(--success); }
.player-online.offline { background: var(--surface-raised); color: var(--muted); }
.reset-btn, .pin-btn { border: 1px solid var(--border); border-radius: 6px; background: var(--surface-raised); color: var(--text-soft); cursor: pointer; }
.reset-btn { padding: 5px 12px; }
.inv-card { width: max-content; max-width: 100%; padding: 12px 14px; }
.inv-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.inv-house, .inv-region, .inv-claim { font-size: 11px; padding: 1px 7px; border-radius: 8px; font-weight: 500; }
.inv-house { background: rgba(192, 132, 252, 0.18); color: #d8b4fe; }
.inv-region { background: rgba(138, 164, 255, 0.18); color: var(--accent); }
.inv-claim { background: rgba(117, 224, 167, 0.16); color: var(--success); }
.pin-btn { margin-left: auto; padding: 3px 10px; font-size: 12px; white-space: nowrap; }
.pin-btn.pinned { background: rgba(138, 164, 255, 0.18); border-color: var(--accent); color: var(--accent); font-weight: 600; }
.error-msg { color: var(--danger); }
</style>
