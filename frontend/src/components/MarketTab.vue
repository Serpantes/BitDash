<template>
  <div class="market-tab">
    <div class="card search-card">
      <div class="search-row">
        <form class="search-form" @submit.prevent="searchPlayer">
          <input
            v-model="query"
            type="text"
            class="search-input"
            placeholder="Имя игрока (мин. 2 символа)..."
            :disabled="searching"
          />
          <button type="submit" class="search-btn" :disabled="searching || query.trim().length < 2">
            {{ searching ? 'Поиск...' : 'Найти' }}
          </button>
        </form>
      </div>

      <ul v-if="results.length && !pickedFromResults" class="results-list">
        <li
          v-for="p in results"
          :key="p.entityId"
          class="result-item"
          @click="pickPlayer(p)"
        >
          <span class="player-name">{{ p.username }}</span>
          <span class="player-meta">{{ p.signedIn ? '🟢 онлайн' : 'офлайн' }}</span>
        </li>
      </ul>

      <div v-if="searchError" class="error-msg">{{ searchError }}</div>

      <div v-if="selectedPlayer" class="selected-bar">
        <div class="selected-info">
          <span class="selected-label">Игрок для рынка:</span>
          <strong>{{ selectedPlayer.username }}</strong>
          <span class="player-id">{{ selectedPlayer.entityId }}</span>
        </div>
        <div class="selected-actions">
          <button type="button" class="ghost-btn" @click="reload" :disabled="loading">
            Обновить список
          </button>
          <button type="button" class="ghost-btn danger" @click="changePlayer">
            Сменить
          </button>
        </div>
      </div>
    </div>

    <div v-if="selectedPlayer && loading" class="card placeholder">Загрузка ордеров...</div>
    <div v-else-if="selectedPlayer && loadError" class="card error-msg">{{ loadError }}</div>
    <div v-else-if="selectedPlayer && enrichedRows.length === 0" class="card placeholder">
      Нет активных ордеров на продажу у этого игрока.
    </div>

    <div v-else-if="selectedPlayer && enrichedRows.length" class="card table-card">
      <div class="table-head">
        <h2 class="table-title">Ордера на продажу</h2>
        <span class="table-hint">
          В ордере BitJita поле цены — за одну штуку; «цена лота» = за шт. × количество.
          Ориентир: сначала средняя за 7 дней (как на BitJita), затем за 24ч, затем мин. цена продажи.
          Процент — насколько ваша цена за шт. выше или ниже этого ориентира.
        </span>
      </div>
      <div class="table-wrap">
        <table class="market-table">
          <thead>
            <tr>
              <th>Предмет</th>
              <th class="num">Кол-во</th>
              <th class="num">Цена лота</th>
              <th class="num">За шт.</th>
              <th class="num cell-market-head">Рынок</th>
              <th>Локация</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, i) in enrichedRows"
              :key="row.entityId ?? row.orderEntityId ?? i"
              class="market-row"
              :class="{ 'market-row--tiered': row.tierRowStyle }"
              :style="row.tierRowStyle"
            >
              <td class="cell-name">
                <ItemCard
                  :name="row.displayName"
                  :icon-asset-name="row.iconAssetName"
                  :item-type="row.itemTypeValue"
                  :tier="row.tier"
                  :rarity-str="row.rarityStr"
                />
              </td>
              <td class="num">{{ row.qtyDisplay }}</td>
              <td class="num mono">{{ row.lotPriceDisplay }}</td>
              <td class="num mono">{{ row.unitPriceDisplay }}</td>
              <td class="num cell-market">
                <div class="ref-stack">
                  <template v-if="row.refValue != null">
                    <div class="ref-kind">{{ row.refKind }}</div>
                    <div class="ref-value">{{ row.refValueDisplay }}</div>
                    <div
                      v-if="row.diffPct != null"
                      class="ref-diff"
                      :class="diffClass(row.diffPct)"
                    >
                      {{ row.diffPct > 0 ? '+' : '' }}{{ row.diffPct.toFixed(1) }}%
                    </div>
                  </template>
                  <span v-else class="muted">—</span>
                </div>
              </td>
              <td class="cell-meta muted">{{ row.location }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else class="card hint-card">
      Выбери игрока выше — покажем все его активные продажи на рынке и сравним цену за штуку с рыночной
      средней (7d / 24h). Выбор сохраняется в этом браузере.
    </div>
  </div>
</template>

<script>
import { ref, computed, watch } from 'vue'
import { fetchBitjita, postBitjita } from '../api'
import { useMarketPlayer } from '../composables/useMarketPlayer'
import { useItemCache } from '../composables/useItemCache'
import { TIER_COLORS, hexToRgb, tierNumber } from '../itemVisuals'
import ItemCard from './ItemCard.vue'

const BULK_CHUNK = 100

function orderItemId(o) {
  return o.itemId ?? o.item_id
}

function orderItemType(o) {
  const t = o.itemType ?? o.item_type
  return t === 1 || t === '1' || t === 'cargo' ? 1 : 0
}

function orderPrice(o) {
  const v = o.price ?? o.priceThreshold
  return v != null ? String(v) : '0'
}

function orderQty(o) {
  const v = o.quantity
  return v != null ? String(v) : '0'
}

function parseBigIntSafe(s) {
  try {
    return BigInt(s || '0')
  } catch {
    return 0n
  }
}

/** API поле price — цена за одну единицу товара (не сумма лота). */
function unitPriceBn(order) {
  const p = parseBigIntSafe(orderPrice(order))
  if (p <= 0n) return null
  return p
}

/** Полная стоимость лота: цена за шт. × количество. */
function lotTotalBn(order) {
  const u = unitPriceBn(order)
  const q = parseBigIntSafe(orderQty(order))
  if (u == null || q <= 0n) return null
  return u * q
}

function formatCoinsBig(bn) {
  if (bn == null) return '—'
  return bn.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '\u202f')
}

function fmtMarket(n) {
  return n.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
}

/** Ориентир для сравнения: как на карточках BitJita — сначала 7d Avg, затем 24h, затем мин. ask. */
function marketReference(stats) {
  const empty = { value: null, kind: '', valueDisplay: '' }
  if (!stats || typeof stats !== 'object') return empty

  const v7 = stats.vwap7d
  if (v7 != null && Number.isFinite(Number(v7))) {
    const n = Number(v7)
    return { value: n, kind: '7d Avg', valueDisplay: fmtMarket(n) }
  }
  const v24 = stats.vwap24h
  if (v24 != null && Number.isFinite(Number(v24))) {
    const n = Number(v24)
    return { value: n, kind: '24h Avg', valueDisplay: fmtMarket(n) }
  }
  const low = stats.lowestSellPrice
  if (low != null && Number.isFinite(Number(low))) {
    const n = Number(low)
    return { value: n, kind: 'Low ask', valueDisplay: fmtMarket(n) }
  }
  return empty
}

function tierRowStyle(tier) {
  const normalized = tierNumber(tier)
  const color = normalized ? TIER_COLORS[normalized] : null
  const rgb = hexToRgb(color)
  if (!color || !rgb) return null
  return {
    '--market-row-tier': color,
    '--market-row-tier-bg': `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.18)`,
    '--market-row-tier-bg-hover': `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.26)`,
  }
}

async function fetchBulkMaps(itemIds, cargoIds) {
  const items = {}
  const cargo = {}
  if (itemIds.length > 0) {
    for (let i = 0; i < itemIds.length; i += BULK_CHUNK) {
      const slice = itemIds.slice(i, i + BULK_CHUNK)
      const data = await postBitjita('/market/prices/bulk', { itemIds: slice, cargoIds: [] })
      Object.assign(items, data?.data?.items ?? {})
    }
  }
  if (cargoIds.length > 0) {
    for (let i = 0; i < cargoIds.length; i += BULK_CHUNK) {
      const slice = cargoIds.slice(i, i + BULK_CHUNK)
      const data = await postBitjita('/market/prices/bulk', { itemIds: [], cargoIds: slice })
      Object.assign(cargo, data?.data?.cargo ?? {})
    }
  }
  return { items, cargo }
}

export default {
  name: 'MarketTab',
  components: { ItemCard },
  setup() {
    const query = ref('')
    const searching = ref(false)
    const searchError = ref(null)
    const results = ref([])
    const pickedFromResults = ref(false)

    const sellOrders = ref([])
    const bulkMaps = ref({ items: {}, cargo: {} })
    const loading = ref(false)
    const loadError = ref(null)

    const { selectedPlayer, setPlayer, clearPlayer } = useMarketPlayer()
    const { resolveItem, prefetch, seedCatalog } = useItemCache()

    async function searchPlayer() {
      const q = query.value.trim()
      if (q.length < 2) return
      searching.value = true
      searchError.value = null
      results.value = []
      pickedFromResults.value = false
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

    function pickPlayer(p) {
      setPlayer(p)
      pickedFromResults.value = true
      query.value = ''
      results.value = []
      searchError.value = null
    }

    function changePlayer() {
      clearPlayer()
      sellOrders.value = []
      bulkMaps.value = { items: {}, cargo: {} }
      pickedFromResults.value = false
      loadError.value = null
    }

    async function loadMarket() {
      if (!selectedPlayer.value) return
      loading.value = true
      loadError.value = null
      sellOrders.value = []
      bulkMaps.value = { items: {}, cargo: {} }
      try {
        const pid = selectedPlayer.value.entityId
        const data = await fetchBitjita(`/players/${pid}/market`)
        seedCatalog(data)
        const sells = Array.isArray(data.sellOrders)
          ? data.sellOrders
          : Array.isArray(data.sell_orders)
            ? data.sell_orders
            : []
        sellOrders.value = sells
        await prefetch([{
          pockets: sells.map((order) => ({
            contents: {
              itemId: orderItemId(order),
              itemType: orderItemType(order),
            },
          })),
        }])

        const itemIds = [
          ...new Set(
            sells
              .filter((o) => orderItemType(o) === 0)
              .map((o) => orderItemId(o))
              .filter((id) => id != null),
          ),
        ].map(Number).filter((n) => Number.isFinite(n))
        const cargoIds = [
          ...new Set(
            sells
              .filter((o) => orderItemType(o) === 1)
              .map((o) => orderItemId(o))
              .filter((id) => id != null),
          ),
        ].map(Number).filter((n) => Number.isFinite(n))

        bulkMaps.value = await fetchBulkMaps(itemIds, cargoIds)
      } catch (e) {
        loadError.value = e.message
      } finally {
        loading.value = false
      }
    }

    function reload() {
      loadMarket()
    }

    function statsFor(order) {
      const idStr = String(orderItemId(order))
      const maps = bulkMaps.value
      return orderItemType(order) === 1 ? maps.cargo[idStr] : maps.items[idStr]
    }

    const enrichedRows = computed(() =>
      sellOrders.value.map((o) => {
        const isCargo = orderItemType(o) === 1
        const stats = statsFor(o)
        const ref = marketReference(stats)
        const unitBn = unitPriceBn(o)
        const lotBn = lotTotalBn(o)
        let diffPct = null
        if (unitBn != null && ref.value != null && ref.value !== 0) {
          diffPct = ((Number(unitBn) - ref.value) / ref.value) * 100
          if (!Number.isFinite(diffPct)) diffPct = null
        }

        const qtyBn = parseBigIntSafe(orderQty(o))

        const name =
          o.itemName ??
          o.item_name ??
          o.name ??
          resolveItem({ itemId: orderItemId(o), itemType: orderItemType(o) }).name
        const meta = resolveItem({ itemId: orderItemId(o), itemType: orderItemType(o) })

        const claim =
          o.claimName ??
          o.claim_name ??
          o.marketClaimName ??
          null
        const region = o.regionName ?? o.region_name ?? null
        const location = [claim, region].filter(Boolean).join(' · ') || '—'

        return {
          ...o,
          entityId: o.entityId ?? o.orderEntityId ?? o.entity_id,
          displayName: name,
          isCargo,
          itemTypeValue: isCargo ? 1 : 0,
          iconAssetName: o.iconAssetName ?? o.icon_asset_name ?? o.itemIconAssetName ?? o.item_icon_asset_name ?? meta.iconAssetName ?? null,
          tier: o.tier ?? o.itemTier ?? o.item_tier ?? meta.tier ?? null,
          tierRowStyle: tierRowStyle(o.tier ?? o.itemTier ?? o.item_tier ?? meta.tier ?? null),
          rarityStr: o.rarityStr ?? o.itemRarityStr ?? o.item_rarity_str ?? meta.rarityStr ?? null,
          qtyDisplay: formatCoinsBig(qtyBn),
          lotPriceDisplay: lotBn != null ? formatCoinsBig(lotBn) : '—',
          unitPriceDisplay: unitBn != null ? formatCoinsBig(unitBn) : '—',
          refKind: ref.kind,
          refValue: ref.value,
          refValueDisplay: ref.value != null ? ref.valueDisplay : '',
          diffPct,
          location,
        }
      }),
    )

    function diffClass(pct) {
      if (pct > 1) return 'diff-above'
      if (pct < -1) return 'diff-below'
      return 'diff-neutral'
    }

    watch(
      selectedPlayer,
      (p) => {
        if (p) loadMarket()
      },
      { immediate: true },
    )

    return {
      query,
      searching,
      searchError,
      results,
      pickedFromResults,
      selectedPlayer,
      loading,
      loadError,
      enrichedRows,
      searchPlayer,
      pickPlayer,
      changePlayer,
      reload,
      diffClass,
    }
  },
}
</script>

<style scoped>
.card {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.24);
}

.market-tab {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.search-card {
  padding: 14px 18px;
}

.search-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.search-form {
  display: flex;
  gap: 8px;
  flex: 1;
  min-width: 220px;
}

.search-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-raised);
  color: var(--text);
  font-size: 14px;
  outline: none;
}
.search-input::placeholder {
  color: var(--muted);
}
.search-input:focus {
  border-color: var(--accent);
}

.search-btn {
  padding: 8px 18px;
  background: var(--accent-strong);
  color: white;
  border: none;
  border-radius: 7px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}
.search-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.results-list {
  list-style: none;
  margin: 10px 0 0;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 7px;
  overflow: hidden;
  background: var(--surface-raised);
}

.result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 9px 14px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.1s;
}

.result-item:not(:last-child) {
  border-bottom: 1px solid var(--border);
}

.result-item:hover {
  background: var(--surface-hover);
}

.player-name {
  font-weight: 500;
  color: var(--text);
}

.player-meta {
  font-size: 12px;
  color: var(--muted);
}

.selected-bar {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
}

.selected-info {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  font-size: 14px;
}

.selected-label {
  color: var(--muted);
}

.player-id {
  font-size: 12px;
  color: var(--muted);
  font-family: ui-monospace, monospace;
}

.selected-actions {
  display: flex;
  gap: 8px;
}

.ghost-btn {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--surface-raised);
  font-size: 13px;
  color: var(--text-soft);
  cursor: pointer;
}

.ghost-btn:hover:not(:disabled) {
  background: var(--surface-hover);
}

.ghost-btn.danger:hover:not(:disabled) {
  border-color: var(--danger);
  color: var(--danger);
}

.placeholder {
  font-size: 14px;
  color: var(--muted);
  padding: 18px 20px;
}

.error-msg {
  font-size: 14px;
  color: var(--danger);
  padding: 14px 18px;
}

.hint-card {
  font-size: 14px;
  color: var(--muted);
  line-height: 1.55;
  padding: 18px 20px;
}

.table-card {
  padding: 0;
  overflow: hidden;
}

.table-head {
  padding: 14px 18px 10px;
  border-bottom: 1px solid var(--border);
}

.table-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
}

.table-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.45;
}

.table-wrap {
  overflow-x: auto;
}

.market-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.market-table th {
  text-align: left;
  padding: 10px 14px;
  background: var(--surface-raised);
  color: var(--muted);
  font-weight: 600;
  white-space: nowrap;
}

.market-table td {
  padding: 10px 14px;
  border-top: 1px solid rgba(106, 104, 126, 0.32);
  color: var(--text-soft);
  vertical-align: top;
  transition: background 0.15s, border-color 0.15s;
}

.market-row--tiered td {
  background: var(--market-row-tier-bg);
  border-top-color: color-mix(in srgb, var(--market-row-tier) 52%, var(--border));
}

.market-row--tiered td:first-child {
  box-shadow: inset 4px 0 0 var(--market-row-tier);
}

.market-row--tiered:hover td {
  background: var(--market-row-tier-bg-hover);
}

.market-table th.num,
.market-table td.num {
  text-align: right;
}

.cell-market-head {
  min-width: 104px;
}

.cell-market {
  vertical-align: middle;
}

.ref-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.ref-kind {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  letter-spacing: 0.02em;
}

.ref-value {
  font-size: 17px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: var(--text);
  line-height: 1.15;
}

.ref-diff {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.mono {
  font-variant-numeric: tabular-nums;
}

.cell-name {
  font-weight: 500;
  color: var(--text);
}

.cargo-badge {
  display: inline-block;
  margin-left: 6px;
  font-size: 10px;
  font-weight: 600;
  background: rgba(248, 198, 109, 0.18);
  color: var(--warning);
  padding: 1px 5px;
  border-radius: 5px;
  vertical-align: middle;
}

.cell-meta {
  font-size: 12px;
  max-width: 220px;
}

.muted {
  color: var(--muted);
}

.diff-above {
  color: var(--danger);
  font-weight: 600;
}

.diff-below {
  color: var(--success);
  font-weight: 600;
}

.diff-neutral {
  color: var(--muted);
}
</style>
