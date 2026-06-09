<template>
  <div class="player-inventory">
    <div class="card search-card">
      <form @submit.prevent="searchPlayer" class="search-form">
        <input v-model="query" type="text" placeholder="Введи имя персонажа..." class="search-input" :disabled="searching" />
        <button type="submit" :disabled="searching || !query.trim()" class="search-btn">{{ searching ? 'Поиск...' : 'Найти' }}</button>
      </form>
      <ul v-if="results.length > 0 && !selectedPlayer" class="results-list">
        <li v-for="p in results" :key="p.entityId" @click="selectPlayer(p)" class="result-item">
          <span class="player-name">{{ p.username }}</span>
          <span class="player-meta">{{ p.signedIn ? 'онлайн' : 'офлайн' }}</span>
        </li>
      </ul>
      <div v-if="searchError" class="error-msg">{{ searchError }}</div>
    </div>

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
            <span v-if="inv._isHousing" class="inv-house">дом</span>
            <span v-if="inv.regionId" class="inv-region">R{{ inv.regionId }}</span>
            <span v-if="inv.claimName" class="inv-claim">{{ inv.claimName }}</span>
            <button
              class="pin-btn"
              :class="{ pinned: isPinned(inv.entityId) }"
              @click="togglePin(inv)"
              :title="isPinned(inv.entityId) ? 'Открепить' : 'Закрепить на дашборде'"
            >{{ isPinned(inv.entityId) ? 'Закреплён' : 'Закрепить' }}</button>
          </div>

          <div v-if="!hasInventoryGrid(inv)" class="empty-inv">пусто</div>
          <div v-else class="inventory-board">
            <div class="inventory-grid" :style="inventoryGridStyle(inv)">
              <div
                v-for="(slot, i) in displayInventorySlots(inv)"
                :key="`${slot.kind}-${i}`"
                class="inventory-slot"
                :class="[`inventory-slot--${slot.kind}`, { 'inventory-slot--empty': !hasContents(slot.pocket) }]"
              >
                <template v-if="hasContents(slot.pocket)">
                  <ItemCard
                    variant="mini"
                    :name="itemMeta(slot.pocket.contents).name"
                    :icon-asset-name="itemMeta(slot.pocket.contents).iconAssetName"
                    :item-type="itemMeta(slot.pocket.contents).type"
                    :tier="itemMeta(slot.pocket.contents).tier"
                    :rarity-str="itemMeta(slot.pocket.contents).rarityStr"
                  />
                  <span class="slot-qty">{{ quantityDisplay(slot.pocket.contents.quantity) }}</span>
                </template>
              </div>
            </div>
          </div>
        </div>
        <div v-if="inventories.length === 0" class="card placeholder">Нет данных об инвентарях</div>
      </template>
    </template>
  </div>
</template>

<script>
import { ref } from 'vue'
import { fetchBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'
import ItemCard from './ItemCard.vue'

export default {
  name: 'PlayerInventory',
  components: { ItemCard },
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
    const { resolveItem, prefetch, seedCatalog } = useItemCache()

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
                inventoryName: detail.buildingName ?? 'House',
                regionId: detail.regionId ?? null,
                claimName: detail.claimName ?? null,
                buildingName: detail.buildingName ?? null,
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
    function inventorySlots(inv) { return inv.pockets ?? [] }
    function hasContents(pocket) {
      const c = pocket?.contents
      return Boolean(c && (c.itemId != null || c.item_id != null))
    }
    function isCargoPocket(pocket) {
      const c = pocket?.contents
      return c?.itemType === 1 || c?.item_type === 'cargo'
    }
    function itemSlots(inv) { return inventorySlots(inv).filter((p) => hasContents(p) && !isCargoPocket(p)) }
    function cargoSlots(inv) { return inventorySlots(inv).filter((p) => hasContents(p) && isCargoPocket(p)) }
    function fillSlots(slots, capacity) {
      if (!capacity) return slots
      if (capacity <= slots.length) return slots.slice(0, capacity)
      return [...slots, ...Array.from({ length: capacity - slots.length }, () => ({ contents: null }))]
    }
    function itemCapacity(inv) {
      const name = inv?.inventoryName ?? ''
      const capacity = Number(inv?.cargoIndex)
      if (Number.isFinite(capacity) && capacity > 0) return capacity
      if (/town bank|ancient bank/i.test(name)) return 64
      if (/lost items chest/i.test(name)) return 32
      return 0
    }
    function cargoCapacity(inv) {
      const name = inv?.inventoryName ?? ''
      if (/town bank|ancient bank|lost items chest/i.test(name)) return 16
      if (/personal cache \(iii\)/i.test(name)) return 24
      if (/personal cache \(ii\)/i.test(name)) return 18
      if (/personal cache(?: \(i\))?$/i.test(name)) return 10
      return 0
    }
    function displayItemSlots(inv) { return fillSlots(itemSlots(inv), itemCapacity(inv)) }
    function displayCargoSlots(inv) { return fillSlots(cargoSlots(inv), cargoCapacity(inv)) }
    function displayInventorySlots(inv) {
      return [
        ...displayItemSlots(inv).map((pocket) => ({ kind: 'item', pocket })),
        ...displayCargoSlots(inv).map((pocket) => ({ kind: 'cargo', pocket })),
      ]
    }
    function optimalGridColumns(slots) {
      const units = slots.reduce((sum, slot) => sum + (slot.kind === 'cargo' ? 2 : 1), 0)
      if (units <= 0) return 1
      const minCols = 4
      const maxCols = 16
      const desiredCols = Math.max(minCols, Math.min(maxCols, Math.round(Math.sqrt(units) * 1.8)))
      const divisors = []
      for (let cols = minCols; cols <= maxCols; cols += 1) if (units % cols === 0) divisors.push(cols)
      if (!divisors.length) return desiredCols
      return divisors.reduce((best, cols) => {
        const bestDiff = Math.abs(best - desiredCols)
        const diff = Math.abs(cols - desiredCols)
        return diff < bestDiff || (diff === bestDiff && cols > best) ? cols : best
      }, divisors[0])
    }
    function inventoryGridStyle(inv) { return { '--inventory-cols': optimalGridColumns(displayInventorySlots(inv)) } }
    function hasInventoryGrid(inv) { return itemCapacity(inv) > 0 || cargoCapacity(inv) > 0 || inventorySlots(inv).length > 0 }
    function itemMeta(contents) { return resolveItem(contents) }
    function quantityDisplay(quantity) { return Number(quantity ?? 0).toLocaleString('ru-RU') }
    function togglePin(inv) { isPinned(inv.entityId) ? unpin(inv.entityId) : pin(selectedPlayer.value, inv) }

    return {
      query, searching, searchError, results, selectedPlayer, inventories, invLoading, invError,
      searchPlayer, selectPlayer, reset, displayInventorySlots, inventoryGridStyle, hasInventoryGrid,
      hasContents, itemMeta, quantityDisplay, isPinned, togglePin,
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
.inv-card { width: max-content; max-width: none; padding: 14px 18px; }
.inv-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.inv-house, .inv-region, .inv-claim { font-size: 11px; padding: 1px 7px; border-radius: 8px; font-weight: 500; }
.inv-house { background: rgba(192, 132, 252, 0.18); color: #d8b4fe; }
.inv-region { background: rgba(138, 164, 255, 0.18); color: var(--accent); }
.inv-claim { background: rgba(117, 224, 167, 0.16); color: var(--success); }
.pin-btn { margin-left: auto; padding: 3px 10px; font-size: 12px; white-space: nowrap; }
.pin-btn.pinned { background: rgba(138, 164, 255, 0.18); border-color: var(--accent); color: var(--accent); font-weight: 600; }
.inventory-board { padding: 8px; background: #252437; border: 2px solid #35344a; border-radius: 10px; }
.inventory-grid { display: grid; grid-template-columns: repeat(var(--inventory-cols), 58px); gap: 6px; justify-content: start; width: max-content; }
.inventory-slot { position: relative; height: 70px; display: flex; align-items: center; justify-content: center; border-radius: 9px; background: linear-gradient(145deg, #4a4960, #2c2b3e); box-shadow: inset 0 0 0 2px #6a687e, inset 0 0 0 4px #2f2e43, 0 1px 2px rgba(0,0,0,.35); }
.inventory-slot--item { width: 58px; }
.inventory-slot--cargo { width: auto; grid-column: span 2; }
.inventory-slot--empty { background: rgba(37, 36, 55, 0.35); border: 3px solid #6a687e; opacity: 1; box-shadow: inset 0 0 0 2px #2f2e43, 0 1px 2px rgba(0,0,0,.35); }
.slot-qty { position: absolute; right: 5px; bottom: 5px; color: #f8fafc; font-size: 13px; font-weight: 800; text-shadow: -1px -1px 0 rgba(0,0,0,.8), 1px -1px 0 rgba(0,0,0,.8), -1px 1px 0 rgba(0,0,0,.8), 1px 1px 0 rgba(0,0,0,.8); pointer-events: none; }
.error-msg { color: var(--danger); }
</style>
