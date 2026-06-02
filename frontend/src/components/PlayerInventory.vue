<template>
  <div class="player-inventory">
    <!-- Search -->
    <div class="card search-card">
      <form @submit.prevent="searchPlayer" class="search-form">
        <input
          v-model="query"
          type="text"
          placeholder="Введи имя персонажа..."
          class="search-input"
          :disabled="searching"
        />
        <button type="submit" :disabled="searching || !query.trim()" class="search-btn">
          {{ searching ? 'Поиск...' : 'Найти' }}
        </button>
      </form>

      <!-- Search results dropdown -->
      <ul v-if="results.length > 0 && !selectedPlayer" class="results-list">
        <li
          v-for="p in results"
          :key="p.entityId"
          @click="selectPlayer(p)"
          class="result-item"
        >
          <span class="player-name">{{ p.username }}</span>
          <span class="player-meta">{{ p.signedIn ? '🟢 онлайн' : 'офлайн' }}</span>
        </li>
      </ul>
      <div v-if="searchError" class="error-msg">{{ searchError }}</div>
    </div>

    <!-- Inventories -->
    <template v-if="selectedPlayer">
      <div class="player-header card">
        <div>
          <span class="player-title">{{ selectedPlayer.username }}</span>
          <span class="player-online" :class="selectedPlayer.signedIn ? 'online' : 'offline'">
            {{ selectedPlayer.signedIn ? 'онлайн' : 'офлайн' }}
          </span>
        </div>
        <button class="reset-btn" @click="reset">✕ Сменить</button>
      </div>

      <div v-if="invLoading" class="card placeholder">Загрузка инвентарей...</div>
      <div v-else-if="invError" class="card error-msg">{{ invError }}</div>
      <template v-else>
        <div v-for="inv in inventories" :key="inv.entityId" class="card inv-card">
          <div class="inv-header">
            <span class="inv-name">{{ inv.inventoryName }}</span>
            <span v-if="inv._isHousing" class="inv-house">🏠 дом</span>
            <span v-if="inv.regionId" class="inv-region">R{{ inv.regionId }}</span>
            <span v-if="inv.claimName" class="inv-claim">{{ inv.claimName }}</span>
            <button
              class="pin-btn"
              :class="{ pinned: isPinned(inv.entityId) }"
              @click="togglePin(inv)"
              :title="isPinned(inv.entityId) ? 'Открепить' : 'Закрепить на дашборде'"
            >{{ isPinned(inv.entityId) ? '📌 Закреплён' : '📌 Закрепить' }}</button>
          </div>

          <div v-if="filledPockets(inv).length === 0" class="empty-inv">пусто</div>
          <div v-else class="pocket-grid">
            <div
              v-for="(pocket, i) in filledPockets(inv)"
              :key="i"
              class="pocket"
            >
              <span class="item-name">
                {{ itemName(pocket.contents) }}
                <span v-if="pocket.contents.itemType === 1" class="cargo-badge">cargo</span>
              </span>
              <span class="item-qty">× {{ pocket.contents.quantity.toLocaleString('ru-RU') }}</span>
            </div>
          </div>
        </div>

        <div v-if="inventories.length === 0" class="card placeholder">
          Нет данных об инвентарях
        </div>
      </template>
    </template>
  </div>
</template>

<script>
import { ref } from 'vue'
import { fetchBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'

export default {
  name: 'PlayerInventory',
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
    const { resolveName, prefetch } = useItemCache()

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
        if (results.value.length === 0) searchError.value = 'Игрок не найден'
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
        // Fetch regular inventories and housing list in parallel
        const [invRes, housingRes] = await Promise.all([
          fetchBitjita(`/players/${player.entityId}/inventories`),
          fetchBitjita(`/players/${player.entityId}/housing`).catch(() => null),
        ])

        const invs = invRes.inventories ?? []

        // Fetch each house's detailed inventory (only non-empty ones)
        const houses = Array.isArray(housingRes) ? housingRes.filter((h) => !h.isEmpty) : []
        const houseInvs = await Promise.all(
          houses.map((h) =>
            fetchBitjita(`/players/${player.entityId}/housing/${h.buildingEntityId}`)
              .then((detail) =>
                // Normalize housing inv to match regular format
                (detail.inventories ?? []).map((hi) => ({
                  entityId: hi.entityId,
                  inventoryName: detail.buildingName ?? 'House',
                  regionId: detail.regionId ?? null,
                  claimName: detail.claimName ?? null,
                  buildingName: detail.buildingName ?? null,
                  // housing uses "inventory" key and item_id/item_type fields — keep as-is,
                  // filledPockets and useItemCache handle both formats
                  pockets: (hi.inventory ?? []).map((p) => ({
                    ...p,
                    contents: p.contents
                      ? {
                          itemId: p.contents.item_id ?? p.contents.itemId,
                          itemType: p.contents.item_type === 'cargo' ? 1 : 0,
                          quantity: p.contents.quantity,
                        }
                      : null,
                  })),
                  _isHousing: true,
                }))
              )
              .catch(() => [])
          )
        )

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

    function filledPockets(inv) {
      return (inv.pockets ?? []).filter(
        (p) => p.contents && (p.contents.itemId != null || p.contents.item_id != null),
      )
    }

    function itemName(contents) {
      return resolveName(contents)
    }

    function togglePin(inv) {
      if (isPinned(inv.entityId)) {
        unpin(inv.entityId)
      } else {
        pin(selectedPlayer.value, inv)
      }
    }

    return {
      query, searching, searchError, results, selectedPlayer,
      inventories, invLoading, invError,
      searchPlayer, selectPlayer, reset,
      filledPockets, itemName,
      isPinned, togglePin,
    }
  },
}
</script>

<style scoped>
.player-inventory { display: flex; flex-direction: column; gap: 14px; }

.search-card { padding: 14px 18px; }
.search-form { display: flex; gap: 8px; }
.search-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 7px;
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}
.search-input:focus { border-color: #3b82f6; }
.search-btn {
  padding: 8px 18px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 7px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}
.search-btn:hover:not(:disabled) { background: #2563eb; }
.search-btn:disabled { opacity: 0.5; cursor: default; }

.results-list {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  border: 1px solid #e2e8f0;
  border-radius: 7px;
  overflow: hidden;
}
.result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 9px 14px;
  cursor: pointer;
  transition: background 0.1s;
  font-size: 14px;
}
.result-item:not(:last-child) { border-bottom: 1px solid #f1f5f9; }
.result-item:hover { background: #f8fafc; }
.player-name { font-weight: 500; color: #0f172a; }
.player-meta { font-size: 12px; color: #64748b; }

.player-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 18px;
}
.player-title { font-size: 17px; font-weight: 700; color: #0f172a; margin-right: 10px; }
.player-online { font-size: 12px; font-weight: 500; padding: 2px 8px; border-radius: 10px; }
.player-online.online { background: #dcfce7; color: #15803d; }
.player-online.offline { background: #f1f5f9; color: #64748b; }
.reset-btn {
  padding: 5px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: white;
  font-size: 13px;
  color: #64748b;
  cursor: pointer;
}
.reset-btn:hover { background: #f8fafc; }

.inv-card { padding: 14px 18px; }
.inv-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.inv-name { font-size: 14px; font-weight: 600; color: #1e293b; }
.inv-house {
  font-size: 11px;
  background: #fdf4ff;
  color: #7e22ce;
  padding: 1px 7px;
  border-radius: 8px;
  font-weight: 500;
}
.inv-region {
  font-size: 11px;
  background: #eff6ff;
  color: #3b82f6;
  padding: 1px 7px;
  border-radius: 8px;
  font-weight: 500;
}
.inv-claim {
  font-size: 11px;
  background: #f0fdf4;
  color: #16a34a;
  padding: 1px 7px;
  border-radius: 8px;
}

.pocket-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.pocket {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 10px;
  background: #f8fafc;
  border-radius: 6px;
  font-size: 13px;
}
.item-name { color: #1e293b; }
.cargo-badge {
  display: inline-block;
  font-size: 10px;
  font-weight: 600;
  background: #fef3c7;
  color: #92400e;
  padding: 1px 5px;
  border-radius: 5px;
  margin-left: 5px;
  vertical-align: middle;
}
.item-qty { color: #64748b; font-variant-numeric: tabular-nums; white-space: nowrap; }

.pin-btn {
  margin-left: auto;
  padding: 3px 10px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: white;
  font-size: 12px;
  color: #64748b;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.pin-btn:hover { background: #eff6ff; border-color: #bfdbfe; color: #3b82f6; }
.pin-btn.pinned { background: #eff6ff; border-color: #bfdbfe; color: #3b82f6; font-weight: 600; }
.empty-inv { font-size: 13px; color: #94a3b8; font-style: italic; }
.placeholder { font-size: 14px; color: #64748b; }
.error-msg { font-size: 14px; color: #b91c1c; }
</style>
