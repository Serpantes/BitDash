<template>
  <div class="pinned-dashboard">
    <div v-if="pinned.length === 0" class="card empty-state">
      <div class="empty-icon">📌</div>
      <div class="empty-title">Нет закреплённых инвентарей</div>
      <div class="empty-hint">Перейди на вкладку <strong>Персонаж</strong>, найди игрока и нажми «Закрепить» рядом с нужным инвентарём.</div>
    </div>

    <template v-else>
      <div class="dash-toolbar">
        <span class="dash-hint">{{ pinned.length }} закреплён{{ plural(pinned.length) }} · обновляется каждые 30 сек</span>
        <button class="refresh-btn" @click="loadAll">&#x21bb; Обновить</button>
      </div>

      <div v-for="item in pinned" :key="item.inventoryId" class="card inv-card">
        <div class="inv-header">
          <div class="inv-meta">
            <span class="inv-name">{{ item.inventoryName }}</span>
            <span v-if="item.regionId" class="inv-region">R{{ item.regionId }}</span>
            <span v-if="item.claimName" class="inv-claim">{{ item.claimName }}</span>
            <span class="inv-player">{{ item.playerName }}</span>
          </div>
          <button class="unpin-btn" @click="unpin(item.inventoryId)" title="Открепить">✕</button>
        </div>

        <div v-if="state[item.inventoryId]?.loading" class="placeholder">Загрузка...</div>
        <div v-else-if="state[item.inventoryId]?.error" class="error-msg">{{ state[item.inventoryId].error }}</div>
        <template v-else>
          <div v-if="!filledPockets(item.inventoryId).length" class="empty-inv">пусто</div>
          <div v-else class="pocket-grid">
            <div v-for="(pocket, i) in filledPockets(item.inventoryId)" :key="i" class="pocket">
              <span class="item-name">
                {{ resolvedName(pocket.contents) }}
                <span v-if="pocket.contents.itemType === 1" class="cargo-badge">cargo</span>
              </span>
              <span class="item-qty">× {{ pocket.contents.quantity.toLocaleString('ru-RU') }}</span>
            </div>
          </div>
        </template>

        <div v-if="state[item.inventoryId]?.updatedAt" class="updated-at">
          Обновлено: {{ state[item.inventoryId].updatedAt }}
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { fetchBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'

export default {
  name: 'PinnedDashboard',
  setup() {
    const { pinned, unpin } = usePinned()

    // state[inventoryId] = { loading, error, pockets, updatedAt }
    const state = reactive({})
    const { resolveName, prefetch } = useItemCache()

    async function loadForPlayer(playerEntityId) {
      const playerPinned = pinned.value.filter((p) => p.playerEntityId === playerEntityId)
      if (!playerPinned.length) return

      playerPinned.forEach((p) => {
        state[p.inventoryId] = { loading: true, error: null, pockets: [], updatedAt: null }
      })

      try {
        const res = await fetchBitjita(`/players/${playerEntityId}/inventories`)
        const invs = res.inventories ?? []

        playerPinned.forEach((pinItem) => {
          const inv = invs.find((i) => i.entityId === pinItem.inventoryId)
          state[pinItem.inventoryId] = {
            loading: false,
            error: inv ? null : 'Инвентарь не найден',
            pockets: inv?.pockets ?? [],
            updatedAt: new Date().toLocaleTimeString('ru-RU'),
          }
        })

        await prefetch(invs)
      } catch (e) {
        playerPinned.forEach((p) => {
          state[p.inventoryId] = { loading: false, error: e.message, pockets: [], updatedAt: null }
        })
      }
    }

    async function loadAll() {
      // Group by player and fetch once per player
      const playerIds = [...new Set(pinned.value.map((p) => p.playerEntityId))]
      await Promise.all(playerIds.map(loadForPlayer))
    }

    function filledPockets(inventoryId) {
      return (state[inventoryId]?.pockets ?? []).filter(
        (p) => p.contents && (p.contents.itemId != null || p.contents.item_id != null),
      )
    }

    function resolvedName(contents) {
      return resolveName(contents)
    }

    function plural(n) {
      if (n % 10 === 1 && n % 100 !== 11) return ''
      if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'о'
      return 'о'
    }

    let timer = null
    onMounted(() => {
      loadAll()
      timer = setInterval(loadAll, 30000)
    })
    onUnmounted(() => clearInterval(timer))

    return { pinned, unpin, state, filledPockets, resolvedName, loadAll, plural }
  },
}
</script>

<style scoped>
.pinned-dashboard { display: flex; flex-direction: column; gap: 14px; }

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 24px;
  text-align: center;
}
.empty-icon { font-size: 36px; }
.empty-title { font-size: 16px; font-weight: 600; color: #1e293b; }
.empty-hint { font-size: 14px; color: #64748b; max-width: 360px; line-height: 1.5; }

.dash-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 2px;
}
.dash-hint { font-size: 13px; color: #94a3b8; }
.refresh-btn {
  font-size: 14px;
  cursor: pointer;
  color: #64748b;
  background: none;
  border: none;
  padding: 4px 8px;
  border-radius: 6px;
  transition: color 0.15s;
}
.refresh-btn:hover { color: #3b82f6; }

.inv-card { padding: 14px 18px; }
.inv-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.inv-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.inv-name { font-size: 14px; font-weight: 600; color: #1e293b; }
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
.inv-player {
  font-size: 12px;
  color: #94a3b8;
  font-style: italic;
}
.unpin-btn {
  border: none;
  background: none;
  font-size: 14px;
  color: #94a3b8;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 5px;
  transition: color 0.15s, background 0.15s;
}
.unpin-btn:hover { color: #ef4444; background: #fff1f2; }

.pocket-grid { display: flex; flex-direction: column; gap: 4px; }
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
.empty-inv { font-size: 13px; color: #94a3b8; font-style: italic; }
.placeholder { font-size: 14px; color: #64748b; }
.error-msg { font-size: 14px; color: #b91c1c; }
.updated-at { font-size: 11px; color: #94a3b8; margin-top: 10px; }
</style>
