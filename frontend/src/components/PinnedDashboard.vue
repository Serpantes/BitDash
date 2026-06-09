<template>
  <div class="pinned-dashboard">
    <div class="dash-toolbar">
      <span class="dash-hint">{{ dashboardUpdatedLabel }}</span>
      <div class="dash-actions">
        <button class="refresh-btn" @click="loadAll" :disabled="pinned.length === 0">&#x21bb; Обновить</button>
        <button class="add-block-btn" @click="openAddWizard">+ Добавить блок</button>
      </div>
    </div>

    <div v-if="pinned.length === 0" class="card empty-state">
      <div class="empty-title">Нет закреплённых инвентарей</div>
      <div class="empty-hint">Нажми «Добавить блок», чтобы закрепить инвентарь игрока или клейма.</div>
    </div>

    <div
      v-else
      ref="gridRef"
      class="dashboard-grid"
      :class="{ 'dashboard-grid--dragging': Boolean(dragState.inventoryId) }"
      :style="{ '--dashboard-cols': dashboardCols }"
      @dragover.prevent
    >
      <div
        v-for="cell in layoutCells"
        :key="`${cell.x}-${cell.y}`"
        class="dashboard-cell"
        :class="dashboardCellClass(cell)"
        :style="dashboardCellStyle(cell)"
        @dragover.prevent="dragOverCell(cell)"
        @drop="dropOnCell($event, cell)"
      ></div>

      <div
        v-for="item in pinned"
        :key="item.inventoryId"
        class="card inv-card"
        :class="dashboardCardClass(item.inventoryId)"
        :style="dashboardCardStyle(item.inventoryId)"
        @dragend="endDrag"
      >
        <div
          class="inv-header"
          draggable="true"
          title="Перетащить блок"
          @dragstart="startDrag($event, item.inventoryId)"
        >
          <div class="inv-meta">
            <span class="inv-name">{{ item.inventoryName }}</span>
            <span v-if="item.regionId" class="inv-region">R{{ item.regionId }}</span>
            <span v-if="item.claimName" class="inv-claim">{{ item.claimName }}</span>
            <span class="inv-player">{{ blockOwnerLabel(item) }}</span>
          </div>
          <button class="unpin-btn" @click="unpin(item.inventoryId)" title="Открепить">✕</button>
        </div>

        <div v-if="state[item.inventoryId]?.loading && !hasInventoryGrid(item.inventoryId)" class="placeholder">Загрузка...</div>
        <div v-else-if="state[item.inventoryId]?.error && !hasInventoryGrid(item.inventoryId)" class="error-msg">{{ state[item.inventoryId].error }}</div>
        <template v-else>
          <div v-if="!hasInventoryGrid(item.inventoryId)" class="empty-inv">пусто</div>
          <div v-else class="inventory-board">
            <div class="inventory-grid" :style="inventoryGridStyle(item.inventoryId)">
              <div
                v-for="(slot, i) in displayInventorySlots(item.inventoryId)"
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
        </template>
      </div>
    </div>

    <div v-if="wizard.open" class="modal-backdrop" @click.self="closeAddWizard">
      <div class="modal-card">
        <div class="modal-head">
          <div>
            <h2 class="modal-title">Добавить блок</h2>
            <p class="modal-subtitle">{{ wizardSubtitle }}</p>
          </div>
          <button class="modal-close" @click="closeAddWizard">✕</button>
        </div>

        <div class="wizard-steps">
          <span class="wizard-step" :class="{ active: wizard.step === 'type', done: wizardStepIndex > 0 }">Тип</span>
          <span class="wizard-step" :class="{ active: wizard.step === 'source', done: wizardStepIndex > 1 }">Источник</span>
          <span class="wizard-step" :class="{ active: wizard.step === 'search', done: wizardStepIndex > 2 }">Поиск</span>
          <span class="wizard-step" :class="{ active: wizard.step === 'inventory' }">Инвентарь</span>
        </div>

        <div v-if="wizard.step === 'type'" class="wizard-panel">
          <button class="choice-card active" @click="chooseBlockType('inventory')">
            <span class="choice-title">Инвентарь</span>
            <span class="choice-text">Блок с содержимым инвентаря игрока, дома или здания клейма.</span>
          </button>
        </div>

        <div v-else-if="wizard.step === 'source'" class="wizard-panel">
          <button class="choice-card" :class="{ active: wizard.sourceType === 'player' }" @click="chooseSourceType('player')">
            <span class="choice-title">Игрок</span>
            <span class="choice-text">Найти персонажа и выбрать один из его инвентарей.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.sourceType === 'claim' }" @click="chooseSourceType('claim')">
            <span class="choice-title">Клейм</span>
            <span class="choice-text">Найти поселение и выбрать инвентарь здания.</span>
          </button>
        </div>

        <div v-else-if="wizard.step === 'search'" class="wizard-panel">
          <form class="wizard-search" @submit.prevent="searchWizard">
            <input
              v-model="wizard.query"
              class="wizard-input"
              type="text"
              :placeholder="wizard.sourceType === 'player' ? 'Имя игрока...' : 'Название клейма...'"
              :disabled="wizard.searching"
            />
            <button class="add-block-btn" type="submit" :disabled="wizard.searching || !wizard.query.trim()">
              {{ wizard.searching ? 'Ищу...' : 'Найти' }}
            </button>
          </form>
          <div v-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div v-if="wizard.results.length" class="wizard-results">
            <button
              v-for="result in wizard.results"
              :key="result.entityId"
              class="wizard-result"
              @click="selectWizardEntity(result)"
            >
              <span class="result-title">{{ wizardResultTitle(result) }}</span>
              <span class="result-meta">{{ wizardResultMeta(result) }}</span>
            </button>
          </div>
        </div>

        <div v-else-if="wizard.step === 'inventory'" class="wizard-panel">
          <div v-if="wizard.loadingInventories" class="placeholder">Загрузка инвентарей...</div>
          <div v-else-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div v-else-if="wizard.inventories.length === 0" class="placeholder">Инвентари не найдены</div>
          <div v-else class="wizard-inventory-list">
            <button
              v-for="inv in wizard.inventories"
              :key="inv.entityId"
              class="wizard-inventory"
              :disabled="isPinned(inv.entityId)"
              @click="addWizardInventory(inv)"
            >
              <span class="result-title">{{ inv.inventoryName }}</span>
              <span class="result-meta">
                {{ wizardInventoryMeta(inv) }}
                <template v-if="isPinned(inv.entityId)"> · уже добавлен</template>
              </span>
            </button>
          </div>
        </div>

        <div class="modal-actions">
          <button class="ghost-btn" type="button" @click="wizardBack" :disabled="wizard.step === 'type'">Назад</button>
          <button class="ghost-btn" type="button" @click="closeAddWizard">Отмена</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, reactive, ref, onMounted, onUnmounted } from 'vue'
import { fetchBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'
import ItemCard from './ItemCard.vue'

export default {
  name: 'PinnedDashboard',
  components: { ItemCard },
  setup() {
    const { pinned, isPinned, unpin, pinBlock, updatePinnedLayout } = usePinned()
    const state = reactive({})
    const gridRef = ref(null)
    const dashboardCols = ref(4)
    const dragState = reactive({ inventoryId: null, overCell: null })
    const wizard = reactive({
      open: false,
      step: 'type',
      blockType: 'inventory',
      sourceType: 'player',
      query: '',
      searching: false,
      error: null,
      results: [],
      selectedEntity: null,
      inventories: [],
      loadingInventories: false,
    })
    const { resolveItem, prefetch, seedCatalog } = useItemCache()
    const wizardSteps = ['type', 'source', 'search', 'inventory']

    const wizardStepIndex = computed(() => wizardSteps.indexOf(wizard.step))
    const wizardSubtitle = computed(() => {
      if (wizard.step === 'type') return 'Выбери, какой вид блока добавить на дашборд.'
      if (wizard.step === 'source') return 'Выбери, откуда взять инвентарь.'
      if (wizard.step === 'search') return wizard.sourceType === 'player' ? 'Найди игрока по имени.' : 'Найди клейм по названию.'
      return 'Выбери конкретный инвентарь для добавления.'
    })
    const dashboardUpdatedLabel = computed(() => {
      const refreshing = pinned.value.some((p) => state[p.inventoryId]?.refreshing)
      const times = pinned.value.map((p) => state[p.inventoryId]?.updatedAt).filter(Boolean)
      if (refreshing && times.length > 0) return `Обновлено: ${times[0]} · обновляем...`
      if (refreshing) return 'Обновляем...'
      if (times.length > 0) return `Обновлено: ${times[0]}`
      return `${pinned.value.length} закреплён${plural(pinned.value.length)}`
    })

    const layoutInfo = computed(() => {
      const occupied = new Map()
      const positions = {}
      let maxY = 0
      pinned.value.forEach((item) => {
        const span = dashboardCardGridSpan(item.inventoryId)
        const saved = item.dashboardLayout ?? {}
        const savedX = Number(saved.x)
        const savedY = Number(saved.y)
        let position = null
        if (
          Number.isInteger(savedX) &&
          Number.isInteger(savedY) &&
          savedX >= 0 &&
          savedY >= 0 &&
          savedX + span <= dashboardCols.value &&
          canOccupy(occupied, savedX, savedY, span)
        ) {
          position = { x: savedX, y: savedY }
        } else {
          position = findFreeCell(occupied, span)
        }
        markOccupied(occupied, position.x, position.y, span, item.inventoryId)
        positions[item.inventoryId] = { ...position, span }
        maxY = Math.max(maxY, position.y)
      })
      return { occupied, positions, rows: Math.max(maxY + 2, 2) }
    })
    const layoutCells = computed(() => {
      const cells = []
      for (let y = 0; y < layoutInfo.value.rows; y += 1) {
        for (let x = 0; x < dashboardCols.value; x += 1) cells.push({ x, y })
      }
      return cells
    })

    function ensureInventoryState(inventoryId) {
      if (!state[inventoryId]) {
        state[inventoryId] = { loading: true, refreshing: false, error: null, inventory: null, pockets: [], updatedAt: null }
      }
      return state[inventoryId]
    }
    function sourceType(pinItem) {
      return pinItem.sourceType ?? 'player'
    }
    function normalizePocket(pocket) {
      if (!pocket?.contents) return { ...pocket, contents: null }
      return {
        ...pocket,
        contents: {
          itemId: pocket.contents.itemId ?? pocket.contents.item_id,
          itemType: pocket.contents.itemType ?? (pocket.contents.item_type === 'cargo' ? 1 : 0),
          quantity: pocket.contents.quantity,
        },
      }
    }
    function normalizeClaimBuilding(building, claim) {
      return {
        entityId: building.entityId,
        inventoryName: building.buildingNickname || building.buildingName || 'Building inventory',
        buildingName: building.buildingName ?? null,
        claimName: claim.name ?? null,
        claimEntityId: claim.entityId,
        regionId: claim.regionId ?? null,
        pockets: (building.inventory ?? []).map(normalizePocket),
      }
    }

    async function loadForPlayer(playerEntityId) {
      const playerPinned = pinned.value.filter((p) => sourceType(p) === 'player' && p.playerEntityId === playerEntityId)
      if (!playerPinned.length) return
      playerPinned.forEach((p) => {
        const invState = ensureInventoryState(p.inventoryId)
        invState[hasInventoryGrid(p.inventoryId) ? 'refreshing' : 'loading'] = true
        invState.error = null
      })
      try {
        const res = await fetchBitjita(`/players/${playerEntityId}/inventories`)
        const invs = res.inventories ?? []
        seedCatalog(res)
        playerPinned.forEach((pinItem) => {
          const inv = invs.find((i) => i.entityId === pinItem.inventoryId)
          const invState = ensureInventoryState(pinItem.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = inv ? null : 'Инвентарь не найден'
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = new Date().toLocaleTimeString('ru-RU')
          }
        })
        await prefetch(invs.map((inv) => ({ ...inv, pockets: (inv.pockets ?? []).slice(0, 200) })))
      } catch (e) {
        playerPinned.forEach((p) => {
          const invState = ensureInventoryState(p.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = e.message
        })
      }
    }
    async function loadForClaim(claimEntityId) {
      const claimPinned = pinned.value.filter((p) => sourceType(p) === 'claim' && p.claimEntityId === claimEntityId)
      if (!claimPinned.length) return
      claimPinned.forEach((p) => {
        const invState = ensureInventoryState(p.inventoryId)
        invState[hasInventoryGrid(p.inventoryId) ? 'refreshing' : 'loading'] = true
        invState.error = null
      })
      try {
        const res = await fetchBitjita(`/claims/${claimEntityId}/inventories`)
        seedCatalog(res)
        const claimMeta = { entityId: claimEntityId, name: claimPinned[0]?.claimName ?? null, regionId: claimPinned[0]?.regionId ?? null }
        const invs = (res.buildings ?? []).map((building) => normalizeClaimBuilding(building, claimMeta))
        claimPinned.forEach((pinItem) => {
          const inv = invs.find((i) => i.entityId === pinItem.inventoryId)
          const invState = ensureInventoryState(pinItem.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = inv ? null : 'Инвентарь не найден'
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = new Date().toLocaleTimeString('ru-RU')
          }
        })
        await prefetch(invs.map((inv) => ({ ...inv, pockets: (inv.pockets ?? []).slice(0, 200) })))
      } catch (e) {
        claimPinned.forEach((p) => {
          const invState = ensureInventoryState(p.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = e.message
        })
      }
    }
    async function loadAll() {
      const playerIds = [...new Set(pinned.value.filter((p) => sourceType(p) === 'player').map((p) => p.playerEntityId))]
      const claimIds = [...new Set(pinned.value.filter((p) => sourceType(p) === 'claim').map((p) => p.claimEntityId))]
      await Promise.all([...playerIds.map(loadForPlayer), ...claimIds.map(loadForClaim)])
    }

    function inventorySlots(inventoryId) { return state[inventoryId]?.pockets ?? [] }
    function inventoryMeta(inventoryId) { return state[inventoryId]?.inventory ?? null }
    function hasContents(pocket) {
      const contents = pocket?.contents
      return Boolean(contents && (contents.itemId != null || contents.item_id != null))
    }
    function isCargoPocket(pocket) {
      const contents = pocket?.contents
      return contents?.itemType === 1 || contents?.item_type === 'cargo'
    }
    function itemSlots(inventoryId) { return inventorySlots(inventoryId).filter((p) => hasContents(p) && !isCargoPocket(p)) }
    function cargoSlots(inventoryId) { return inventorySlots(inventoryId).filter((p) => hasContents(p) && isCargoPocket(p)) }
    function fillSlots(slots, capacity) {
      if (!capacity) return slots
      const visibleSlots = slots.slice(0, capacity)
      if (capacity <= visibleSlots.length) return visibleSlots
      return [...visibleSlots, ...Array.from({ length: capacity - visibleSlots.length }, () => ({ contents: null }))]
    }
    function itemCapacity(inventoryId) {
      const meta = inventoryMeta(inventoryId)
      const name = `${meta?.inventoryName ?? ''} ${meta?.buildingName ?? ''}`
      const capacity = Number(meta?.cargoIndex)
      if (Number.isFinite(capacity) && capacity > 0) return capacity
      return buildingItemCapacity(name)
    }
    function cargoCapacity(inventoryId) {
      const meta = inventoryMeta(inventoryId)
      const name = `${meta?.inventoryName ?? ''} ${meta?.buildingName ?? ''}`
      if (/personal cache \(iii\)/i.test(name)) return 24
      if (/personal cache \(ii\)/i.test(name)) return 18
      if (/personal cache(?: \(i\))?$/i.test(name)) return 10
      return buildingCargoCapacity(name)
    }
    function tierIndex(name) {
      const tiers = ['rough', 'simple', 'sturdy', 'fine', 'exquisite', 'peerless', 'ornate', 'pristine', 'magnificent', 'flawless']
      const lower = name.toLowerCase()
      return tiers.findIndex((tier) => lower.includes(tier))
    }
    function tierValue(name, values) {
      const index = tierIndex(name)
      return index >= 0 ? values[index] : 0
    }
    function buildingItemCapacity(name) {
      if (/ancient bank|town bank/i.test(name)) return 64
      if (/lost items chest/i.test(name)) return 32
      if (/gothic item storage/i.test(name)) return 72
      if (/skillful leathercraft item storage/i.test(name)) return 60
      if (/cozy cottage item storage/i.test(name)) return 48
      if (/talented woodcraft item storage/i.test(name)) return 36
      if (/logcraft item storage/i.test(name)) return 30
      if (/wicker item storage/i.test(name)) return 24
      if (/item recovery pickup/i.test(name)) return 14
      if (/large (stone )?chest/i.test(name)) return tierValue(name, [18, 24, 30, 36, 42, 48, 54, 60, 66, 72])
      if (/(stone )?chest/i.test(name)) return tierValue(name, [9, 12, 15, 18, 21, 24, 27, 30, 33, 36])
      if (/barter (counter|stall)/i.test(name)) return tierValue(name, [20, 0, 40, 0, 60, 0, 0, 0, 0, 0])
      return 0
    }
    function buildingCargoCapacity(name) {
      if (/ancient bank|town bank|lost items chest/i.test(name)) return 16
      if (/gothic cargo bin/i.test(name)) return 60
      if (/skillful leathercraft cargo bin/i.test(name)) return 48
      if (/cozy cottage cargo bin/i.test(name)) return 36
      if (/talented woodcraft cargo bin/i.test(name)) return 24
      if (/logcraft cargo bin/i.test(name)) return 18
      if (/wicker cargo bin/i.test(name)) return 12
      if (/wicker basket/i.test(name)) return 6
      if (/small stockpile|supplies stockpile/i.test(name)) return 4
      if (/carpentry stockpile/i.test(name)) return tierValue(name, [2, 4, 6, 8, 10, 12, 0, 0, 0, 0])
      if (/fishing stockpile/i.test(name)) return tierValue(name, [1, 2, 3, 4, 5, 6, 0, 0, 0, 0])
      if (/hunting stockpile/i.test(name)) return tierValue(name, [1, 2, 6, 9, 10, 11, 0, 0, 0, 0])
      if (/quarry stockpile/i.test(name)) return tierValue(name, [3, 5, 8, 11, 13, 16, 0, 0, 0, 0])
      if (/stockpile/i.test(name)) return tierValue(name, [6, 12, 18, 24, 30, 36, 42, 48, 54, 60])
      if (/barter (counter|stall)/i.test(name)) return tierValue(name, [6, 0, 12, 0, 18, 0, 0, 0, 0, 0])
      return 0
    }
    function displayItemSlots(inventoryId) { return fillSlots(itemSlots(inventoryId), itemCapacity(inventoryId)) }
    function displayCargoSlots(inventoryId) { return fillSlots(cargoSlots(inventoryId), cargoCapacity(inventoryId)) }
    function displayInventorySlots(inventoryId) {
      return [
        ...displayItemSlots(inventoryId).map((pocket) => ({ kind: 'item', pocket })),
        ...displayCargoSlots(inventoryId).map((pocket) => ({ kind: 'cargo', pocket })),
      ]
    }
    function dashboardCardSpan(inventoryId) {
      const itemCount = displayItemSlots(inventoryId).length
      const cargoCount = displayCargoSlots(inventoryId).length
      if (cargoCount > 0) return 2
      if (itemCount > 64) return 2
      return 1
    }
    function dashboardCardGridSpan(inventoryId) { return Math.min(dashboardCardSpan(inventoryId), dashboardCols.value) }
    function cellKey(x, y) { return `${x}:${y}` }
    function canOccupy(occupied, x, y, span) {
      if (x < 0 || y < 0 || x + span > dashboardCols.value) return false
      for (let offset = 0; offset < span; offset += 1) {
        if (occupied.get(cellKey(x + offset, y))) return false
      }
      return true
    }
    function markOccupied(occupied, x, y, span, inventoryId = true) {
      for (let offset = 0; offset < span; offset += 1) occupied.set(cellKey(x + offset, y), inventoryId)
    }
    function findFreeCell(occupied, span) {
      for (let y = 0; y < 100; y += 1) {
        for (let x = 0; x <= dashboardCols.value - span; x += 1) {
          if (canOccupy(occupied, x, y, span)) return { x, y }
        }
      }
      return { x: 0, y: 0 }
    }
    function occupiedWithout(inventoryId) {
      const occupied = new Map()
      Object.entries(layoutInfo.value.positions).forEach(([id, pos]) => {
        if (id === String(inventoryId)) return
        markOccupied(occupied, pos.x, pos.y, pos.span, id)
      })
      return occupied
    }
    function canDropAt(inventoryId, cell) {
      if (!inventoryId) return false
      return canOccupy(occupiedWithout(inventoryId), cell.x, cell.y, dashboardCardGridSpan(inventoryId))
    }
    function dashboardCardStyle(inventoryId) {
      const pos = layoutInfo.value.positions[inventoryId]
      if (!pos) return {}
      return { gridColumn: `${pos.x + 1} / span ${pos.span}`, gridRow: `${pos.y + 1} / span 1` }
    }
    function dashboardCellStyle(cell) { return { gridColumn: `${cell.x + 1} / span 1`, gridRow: `${cell.y + 1} / span 1` } }
    function dashboardCellClass(cell) {
      const isOccupied = layoutInfo.value.occupied.has(cellKey(cell.x, cell.y))
      const isOver = dragState.overCell?.x === cell.x && dragState.overCell?.y === cell.y
      return {
        'dashboard-cell--occupied': isOccupied,
        'dashboard-cell--drop-ok': isOver && canDropAt(dragState.inventoryId, cell),
        'dashboard-cell--drop-blocked': isOver && dragState.inventoryId && !canDropAt(dragState.inventoryId, cell),
      }
    }
    function dashboardCardClass(inventoryId) {
      return {
        'inv-card--large': dashboardCardSpan(inventoryId) === 2,
        'inv-card--small': dashboardCardSpan(inventoryId) === 1,
        'inv-card--dragging': dragState.inventoryId === inventoryId,
      }
    }
    function optimalGridColumns(slots, maxCols = 16) {
      const units = slots.reduce((sum, slot) => sum + (slot.kind === 'cargo' ? 2 : 1), 0)
      if (units <= 0) return 1
      const minCols = 4
      const desiredCols = Math.max(minCols, Math.min(maxCols, Math.round(Math.sqrt(units) * 1.8)))
      const divisors = []
      for (let cols = minCols; cols <= maxCols; cols += 1) if (units % cols === 0) divisors.push(cols)
      if (divisors.length === 0) return desiredCols
      return divisors.reduce((best, cols) => {
        const bestDiff = Math.abs(best - desiredCols)
        const diff = Math.abs(cols - desiredCols)
        return diff < bestDiff || (diff === bestDiff && cols > best) ? cols : best
      }, divisors[0])
    }
    function inventoryGridStyle(inventoryId) {
      const maxCols = dashboardCardSpan(inventoryId) === 2 ? 16 : 8
      return { '--inventory-cols': optimalGridColumns(displayInventorySlots(inventoryId), maxCols) }
    }
    function hasInventoryGrid(inventoryId) {
      return itemCapacity(inventoryId) > 0 || cargoCapacity(inventoryId) > 0 || inventorySlots(inventoryId).length > 0
    }
    function itemMeta(contents) { return resolveItem(contents) }
    function quantityDisplay(quantity) { return Number(quantity ?? 0).toLocaleString('ru-RU') }
    function blockOwnerLabel(item) {
      if (sourceType(item) === 'claim') return item.claimName ? `Клейм: ${item.claimName}` : 'Клейм'
      return item.playerName ?? 'Игрок'
    }
    function plural(n) {
      if (n % 10 === 1 && n % 100 !== 11) return ''
      if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'о'
      return 'о'
    }
    function startDrag(event, inventoryId) {
      dragState.inventoryId = inventoryId
      dragState.overCell = null
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', String(inventoryId))
    }
    function dragOverCell(cell) {
      if (!dragState.inventoryId) return
      dragState.overCell = cell
    }
    function dropOnCell(event, cell) {
      event.preventDefault()
      if (dragState.inventoryId && canDropAt(dragState.inventoryId, cell)) {
        updatePinnedLayout(dragState.inventoryId, { x: cell.x, y: cell.y })
      }
      endDrag()
    }
    function endDrag() {
      dragState.inventoryId = null
      dragState.overCell = null
    }

    function resetWizard() {
      Object.assign(wizard, {
        step: 'type',
        blockType: 'inventory',
        sourceType: 'player',
        query: '',
        searching: false,
        error: null,
        results: [],
        selectedEntity: null,
        inventories: [],
        loadingInventories: false,
      })
    }
    function openAddWizard() { resetWizard(); wizard.open = true }
    function closeAddWizard() { wizard.open = false; resetWizard() }
    function chooseBlockType(type) { wizard.blockType = type; wizard.step = 'source' }
    function chooseSourceType(type) {
      wizard.sourceType = type
      wizard.query = ''
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      wizard.step = 'search'
    }
    function wizardBack() {
      const index = wizardSteps.indexOf(wizard.step)
      if (index > 0) wizard.step = wizardSteps[index - 1]
    }
    async function searchWizard() {
      const q = wizard.query.trim()
      if (!q) return
      wizard.searching = true
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      try {
        const res = await fetchBitjita(wizard.sourceType === 'player' ? '/players' : '/claims', { q })
        wizard.results = wizard.sourceType === 'player' ? (res.players ?? []) : (res.claims ?? [])
        if (wizard.results.length === 0) wizard.error = wizard.sourceType === 'player' ? 'Игрок не найден' : 'Клейм не найден'
      } catch (e) {
        wizard.error = e.message
      } finally {
        wizard.searching = false
      }
    }
    async function selectWizardEntity(entity) {
      wizard.selectedEntity = entity
      wizard.error = null
      wizard.loadingInventories = true
      wizard.inventories = []
      wizard.step = 'inventory'
      try {
        wizard.inventories = wizard.sourceType === 'player'
          ? await loadWizardPlayerInventories(entity)
          : await loadWizardClaimInventories(entity)
      } catch (e) {
        wizard.error = e.message
      } finally {
        wizard.loadingInventories = false
      }
    }
    async function loadWizardPlayerInventories(player) {
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
              pockets: (hi.inventory ?? []).map(normalizePocket),
              _isHousing: true,
            }))
          })
          .catch(() => [])
      ))
      return [...invs, ...houseInvs.flat()]
    }
    async function loadWizardClaimInventories(claim) {
      const res = await fetchBitjita(`/claims/${claim.entityId}/inventories`)
      seedCatalog(res)
      return (res.buildings ?? [])
        .filter((building) => (building.inventory ?? []).length > 0 || isStorageBuilding(building))
        .map((building) => normalizeClaimBuilding(building, claim))
    }
    function isStorageBuilding(building) {
      const name = `${building.buildingName ?? ''} ${building.buildingNickname ?? ''}`
      return /bank|chest|stockpile|storage|cargo|cache|barter|basket/i.test(name)
    }
    function addWizardInventory(inv) {
      if (wizard.sourceType === 'player') {
        pinBlock({
          sourceType: 'player',
          inventoryId: inv.entityId,
          playerEntityId: wizard.selectedEntity.entityId,
          playerName: wizard.selectedEntity.username,
          inventoryName: inv.inventoryName,
          regionId: inv.regionId ?? null,
          claimName: inv.claimName ?? null,
        })
      } else {
        pinBlock({
          sourceType: 'claim',
          inventoryId: inv.entityId,
          claimEntityId: wizard.selectedEntity.entityId,
          claimName: wizard.selectedEntity.name,
          regionId: wizard.selectedEntity.regionId ?? inv.regionId ?? null,
          inventoryName: inv.inventoryName,
        })
      }
      closeAddWizard()
      loadAll()
    }
    function wizardResultTitle(result) { return wizard.sourceType === 'player' ? result.username : result.name }
    function wizardResultMeta(result) {
      if (wizard.sourceType === 'player') return result.signedIn ? 'онлайн' : 'офлайн'
      return [result.regionName ? `R${result.regionId} · ${result.regionName}` : (result.regionId ? `R${result.regionId}` : null), result.tier ? `T${result.tier}` : null].filter(Boolean).join(' · ')
    }
    function wizardInventoryMeta(inv) {
      const parts = []
      if (inv.regionId) parts.push(`R${inv.regionId}`)
      if (inv.claimName) parts.push(inv.claimName)
      if (inv.buildingName && inv.buildingName !== inv.inventoryName) parts.push(inv.buildingName)
      parts.push(`${(inv.pockets ?? []).length} заполн.`)
      return parts.join(' · ')
    }

    let timer = null
    let resizeObserver = null
    function updateDashboardCols() {
      if (!gridRef.value) return
      const containerWidth = gridRef.value.parentElement?.clientWidth ?? gridRef.value.clientWidth
      const viewportWidth = window.innerWidth - 32
      const width = Math.max(containerWidth, viewportWidth)
      const cell = 380
      const gap = 14
      dashboardCols.value = Math.max(1, Math.floor((width + gap) / (cell + gap)) || 1)
    }
    onMounted(() => {
      updateDashboardCols()
      resizeObserver = new ResizeObserver(updateDashboardCols)
      if (gridRef.value) resizeObserver.observe(gridRef.value)
      loadAll()
      timer = setInterval(loadAll, 30000)
    })
    onUnmounted(() => {
      clearInterval(timer)
      resizeObserver?.disconnect()
    })

    return {
      pinned, isPinned, unpin, state, dragState, wizard, wizardStepIndex, wizardSubtitle,
      gridRef, dashboardCols, dashboardUpdatedLabel, layoutCells,
      displayInventorySlots, dashboardCardClass, dashboardCardStyle, dashboardCellClass, dashboardCellStyle,
      inventoryGridStyle, hasInventoryGrid, hasContents, itemMeta, quantityDisplay, blockOwnerLabel,
      loadAll, startDrag, dragOverCell, dropOnCell, endDrag,
      openAddWizard, closeAddWizard, chooseBlockType, chooseSourceType, searchWizard,
      selectWizardEntity, addWizardInventory, wizardBack, wizardResultTitle, wizardResultMeta, wizardInventoryMeta,
    }
  },
}
</script>

<style scoped>
.pinned-dashboard {
  --dashboard-cell: 380px;
  --dashboard-row: 446px;
  --dash-slot: 38px;
  --dash-slot-height: 50px;
  --dash-gap: 3px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}
.empty-state { padding: 40px 24px; text-align: center; }
.empty-title { font-size: 16px; font-weight: 600; color: var(--text); }
.empty-hint { font-size: 14px; color: var(--muted); line-height: 1.5; }
.dash-toolbar { display: flex; justify-content: space-between; align-items: center; padding: 0 2px; }
.dash-hint { font-size: 13px; color: var(--muted); }
.dash-actions { display: flex; gap: 8px; align-items: center; }
.refresh-btn {
  font-size: 14px;
  cursor: pointer;
  color: var(--muted);
  background: none;
  border: none;
  padding: 4px 8px;
  border-radius: 6px;
  transition: color 0.15s;
}
.refresh-btn:hover { color: var(--accent); }
.refresh-btn:disabled { opacity: 0.45; cursor: default; }
.add-block-btn {
  padding: 7px 12px;
  border: 1px solid var(--accent);
  border-radius: 7px;
  background: rgba(138, 164, 255, 0.16);
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(var(--dashboard-cols), var(--dashboard-cell));
  grid-auto-rows: var(--dashboard-row);
  gap: 14px;
  align-items: stretch;
  width: calc((var(--dashboard-cell) * var(--dashboard-cols)) + (14px * (var(--dashboard-cols) - 1)));
  max-width: none;
}
.dashboard-cell {
  min-width: 0;
  min-height: 0;
  border: 1px dashed transparent;
  border-radius: 10px;
  background: transparent;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
}
.dashboard-grid--dragging .dashboard-cell { border-color: rgba(106, 104, 126, 0.38); background: rgba(37, 36, 55, 0.16); }
.dashboard-cell--occupied { pointer-events: none; opacity: 0; }
.dashboard-cell--drop-ok { border-color: var(--accent); background: rgba(138, 164, 255, 0.16); box-shadow: inset 0 0 0 2px rgba(138, 164, 255, 0.2); }
.dashboard-cell--drop-blocked { border-color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.inv-card {
  z-index: 1;
  width: auto;
  height: 100%;
  min-width: 0;
  max-width: none;
  padding: 14px 18px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: border-color 0.15s, box-shadow 0.15s, opacity 0.15s, transform 0.15s;
}
.inv-card--dragging { opacity: 0.55; transform: scale(0.995); }
.inv-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; cursor: grab; user-select: none; }
.inv-header:active { cursor: grabbing; }
.inv-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.inv-name { font-size: 14px; font-weight: 600; color: var(--text); }
.inv-region { font-size: 11px; background: rgba(138, 164, 255, 0.18); color: var(--accent); padding: 1px 7px; border-radius: 8px; font-weight: 500; }
.inv-claim { font-size: 11px; background: rgba(117, 224, 167, 0.16); color: var(--success); padding: 1px 7px; border-radius: 8px; }
.inv-player { font-size: 12px; color: var(--muted); font-style: italic; }
.unpin-btn { border: none; background: none; font-size: 14px; color: var(--muted); cursor: pointer; padding: 2px 6px; border-radius: 5px; }
.unpin-btn:hover { color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.inventory-board { flex: 1; min-height: 0; padding: 8px; background: #252437; border: 2px solid #35344a; border-radius: 10px; width: fit-content; max-width: 100%; overflow: hidden; }
.inventory-grid { display: grid; grid-template-columns: repeat(var(--inventory-cols), var(--dash-slot)); gap: var(--dash-gap); justify-content: start; width: max-content; max-width: 100%; }
.inventory-slot { position: relative; height: var(--dash-slot-height); display: flex; align-items: center; justify-content: center; border-radius: 9px; background: linear-gradient(145deg, #4a4960, #2c2b3e); box-shadow: inset 0 0 0 2px #6a687e, inset 0 0 0 4px #2f2e43, 0 1px 2px rgba(0, 0, 0, 0.35); }
.inventory-slot--item { width: var(--dash-slot); }
.inventory-slot--cargo { width: auto; grid-column: span 2; }
.inventory-slot--empty { background: rgba(37, 36, 55, 0.35); border: 3px solid #6a687e; opacity: 1; box-shadow: inset 0 0 0 2px #2f2e43, 0 1px 2px rgba(0, 0, 0, 0.35); }
.slot-qty { position: absolute; right: 5px; bottom: 5px; color: #f8fafc; font-size: 13px; font-weight: 800; line-height: 1; text-shadow: -1px -1px 0 rgba(0,0,0,.8), 1px -1px 0 rgba(0,0,0,.8), -1px 1px 0 rgba(0,0,0,.8), 1px 1px 0 rgba(0,0,0,.8); pointer-events: none; }
.inventory-slot :deep(.item-card--mini .item-card__icon) { width: 34px; height: 44px; flex-basis: 34px; border-radius: 7px; }
.inventory-slot--cargo :deep(.item-card--mini .item-card__icon) { width: 74px; flex-basis: 74px; }
.inventory-slot :deep(.item-card--mini.item-card--rarity .item-card__icon) { border-width: 3px; }
.inventory-slot :deep(.item-card--mini .item-card__fallback) { font-size: 9px; }
.inventory-slot :deep(.item-card--mini .item-card__image) { width: 84%; height: 84%; }
.empty-inv, .placeholder { font-size: 13px; color: var(--muted); font-style: italic; }
.error-msg, .wizard-error { font-size: 13px; color: var(--danger); }
.modal-backdrop { position: fixed; inset: 0; z-index: 50; display: flex; align-items: center; justify-content: center; padding: 24px; background: rgba(10, 9, 18, 0.72); backdrop-filter: blur(4px); }
.modal-card { width: min(720px, 100%); max-height: min(760px, calc(100vh - 48px)); display: flex; flex-direction: column; padding: 18px; background: var(--surface); border: 1px solid var(--border); border-radius: 14px; box-shadow: 0 24px 80px rgba(0,0,0,.45); }
.modal-head { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
.modal-title { margin: 0; font-size: 18px; color: var(--text); }
.modal-subtitle { margin: 5px 0 0; font-size: 13px; color: var(--muted); }
.modal-close { border: none; background: transparent; color: var(--muted); cursor: pointer; border-radius: 6px; padding: 4px 7px; }
.wizard-steps { display: flex; gap: 8px; margin: 18px 0 14px; flex-wrap: wrap; }
.wizard-step { padding: 4px 9px; border-radius: 999px; background: var(--surface-raised); color: var(--muted); font-size: 12px; font-weight: 600; }
.wizard-step.active { background: rgba(138, 164, 255, 0.2); color: var(--accent); }
.wizard-step.done { color: var(--success); }
.wizard-panel { min-height: 220px; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding-right: 4px; }
.choice-card, .wizard-result, .wizard-inventory { width: 100%; text-align: left; padding: 12px 14px; border: 1px solid var(--border); border-radius: 10px; background: var(--surface-raised); color: var(--text); cursor: pointer; }
.choice-card:hover, .wizard-result:hover, .wizard-inventory:hover:not(:disabled) { border-color: var(--accent); background: var(--surface-hover); }
.choice-card.active { border-color: var(--accent); background: rgba(138, 164, 255, 0.14); }
.choice-title, .result-title { display: block; font-size: 14px; font-weight: 700; color: var(--text); }
.choice-text, .result-meta { display: block; margin-top: 4px; font-size: 12px; color: var(--muted); line-height: 1.4; }
.wizard-search { display: flex; gap: 8px; }
.wizard-input { flex: 1; min-width: 0; padding: 8px 11px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); color: var(--text); outline: none; }
.wizard-results, .wizard-inventory-list { display: flex; flex-direction: column; gap: 8px; }
.wizard-inventory:disabled { opacity: 0.5; cursor: default; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--border); }
.ghost-btn { padding: 7px 12px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); color: var(--text-soft); cursor: pointer; }
@media (max-width: 840px) {
  .pinned-dashboard { --dashboard-cell: calc(100vw - 32px); --dashboard-row: 446px; }
  .inv-card--large { grid-column: span 1; }
}
</style>
