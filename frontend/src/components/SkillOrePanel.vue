<template>
  <div class="ore">
    <div v-if="error && !data" class="bc-error">{{ error }}</div>
    <div v-else-if="data && !data.setup?.tool" class="bc-muted-note">{{ t('ore.noHammer') }}</div>
    <template v-else-if="data">
      <div v-if="picking" class="ore-picker">
        <div class="ore-picker-head">
          <span class="ore-picker-title">{{ t('ore.pickTitle') }}</span>
          <label class="ore-picker-all"><input v-model="showAllInventories" type="checkbox"> {{ t('ore.pickShowAll') }}</label>
          <button type="button" class="bc-btn ore-picker-done" @click="picking = false">{{ t('ore.pickDone') }}</button>
        </div>
        <div v-if="!stock" class="bc-muted-note">{{ stockError || t('ore.pickLoading') }}</div>
        <div v-else-if="!pickerGroups.length" class="bc-muted-note">{{ t('ore.pickEmpty') }}</div>
        <div v-else class="ore-picker-list">
          <section class="ore-picker-group">
            <label class="ore-picker-row ore-picker-crafts">
              <input type="checkbox" :checked="trackCrafts" @change="$emit('update:track-crafts', !trackCrafts)">
              <span class="ore-picker-name">{{ t('ore.pickCrafts') }}</span>
              <span class="ore-picker-ore">{{ craftsSummary || t('ore.pickNoCrafts') }}</span>
            </label>
          </section>
          <section v-for="group in pickerGroups" :key="group.key" class="ore-picker-group">
            <div class="ore-picker-place">{{ group.title }}</div>
            <label v-for="inv in group.inventories" :key="inv.id" class="ore-picker-row">
              <input type="checkbox" :checked="trackedSet.has(inv.id)" @change="toggleTracked(inv.id)">
              <span class="ore-picker-name">{{ inv.name }}</span>
              <span class="ore-picker-ore">{{ oreSummary(inv) }}</span>
            </label>
          </section>
        </div>
      </div>
      <div v-else-if="xpLeft <= 0" class="bc-muted-note">{{ t('ore.reached', { level: targetLevel }) }}</div>
      <template v-else>
        <div v-if="selected" class="ore-hero" :title="rowTitle(selected)">
          <div class="ore-hero-figure">
            <span class="ore-hero-value">{{ formatNumber(selected.chunks) }}</span>
            <span class="ore-hero-label">{{ t('ore.heroLeft', { chunk: selected.chunkName }) }}</span>
          </div>
          <dl class="ore-hero-facts">
            <dt>{{ t('ore.heroHave') }}</dt>
            <dd :title="stockTitle(selected)">{{ tracking ? formatNumber(selected.haveChunks) : '—' }}</dd>
            <dt>{{ t('ore.heroTotal', { level: targetLevel }) }}</dt>
            <dd>{{ formatNumber(selected.chunksTotal) }}</dd>
            <dt>{{ t('ore.heroMoney') }}</dt>
            <dd :title="moneyLabel(selected)">{{ moneyText(selected) }}</dd>
          </dl>
          <div class="ore-hero-side">
            T{{ selected.tier }} · {{ formatNumber(selected.ingots) }} × {{ selected.ingotName }}<template v-if="selected.flux"> · {{ formatNumber(selected.flux) }} × {{ t('ore.flux') }}</template> · {{ formatNumber(selected.ingots) }} × {{ selected.logName }}
          </div>
        </div>

        <div class="ore-table-wrap">
          <table class="ore-table">
            <thead>
              <tr>
                <th class="ore-col-name">{{ t('ore.colTier') }}</th>
                <th class="ore-main" :title="t('ore.colChunksTitle')">{{ t('ore.colChunks') }}</th>
                <th class="ore-main" :title="t('ore.colMoneyTitle')">{{ t('ore.colMoney') }}</th>
                <th class="ore-gap" :title="t('ore.colPiecesTitle')">{{ t('ore.colPieces') }}</th>
                <th>{{ t('ore.colIngots') }}</th>
                <th>{{ t('ore.colFlux') }}</th>
                <th :title="t('ore.colTimeTitle')">{{ t('ore.colTime') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rows"
                :key="row.tier"
                :class="{ 'ore-row--selected': row.tier === selected?.tier, 'ore-row--locked': row.locked }"
                :title="rowTitle(row)"
                @click="!row.locked && $emit('update:tier', row.tier)"
              >
                <td class="ore-col-name">
                  <span class="ore-tier">T{{ row.tier }}</span>
                  <span class="ore-name">{{ row.ingotName.replace(/ Ingot$/, '') }}</span>
                  <span v-if="row.locked" class="ore-lock">{{ lockText(row) }}</span>
                </td>
                <td class="ore-main" :title="stockTitle(row)">
                  {{ formatNumber(row.chunks) }}
                  <span v-if="row.haveChunks" class="ore-have">{{ t('ore.haveOf', { have: formatNumber(row.haveChunks), total: formatNumber(row.chunksTotal) }) }}</span>
                </td>
                <td class="ore-main">{{ moneyText(row) }}</td>
                <td class="ore-gap" :title="stockTitle(row)">
                  {{ formatNumber(row.pieces) }}
                  <span v-if="row.havePieces" class="ore-have">{{ t('ore.haveOf', { have: formatNumber(row.havePieces), total: formatNumber(row.piecesTotal) }) }}</span>
                </td>
                <td>{{ formatNumber(row.ingots) }}</td>
                <td>{{ row.flux ? formatNumber(row.flux) : '—' }}</td>
                <td>{{ formatPlayTime(row.seconds) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <div class="ore-foot">
        <button type="button" class="ore-track-btn" :class="{ 'ore-track-btn--on': tracking }" @click="picking = !picking">
          {{ trackLabel }}
        </button>
        <span>×{{ formatNumber(data.setup.xpRate, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} XP</span>
        <span :title="t('ore.progressTitle')">{{ t('ore.progressShort', { n: formatNumber(data.setup.progressPerSwing), tool: data.setup.tool.name, power: data.setup.tool.power, skill: data.setup.skillPower }) }}</span>
        <span>{{ t('ore.leftShort', { xp: formatXpCompact(xpLeft), level: targetLevel }) }}</span>
        <span v-if="marketAge">{{ t('ore.marketAge', { age: marketAge }) }}</span>
      </div>
    </template>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { fetchSmithingOre, fetchSmithingStock } from '../api'
import { formatNumber, t } from '../i18n'
import { formatXpCompact, formatXpShort } from '../skillProgress'
import { formatEta, formatPlayTime } from '../xpHistory'
import { orePlan, readCachedOreData, trackedStock, writeCachedOreData } from '../smithingOre'

// The backend keeps setups and order books fresh within minutes; polling slower is enough.
const REFRESH_MS = 5 * 60 * 1000
// Tracked inventories change as ore is gathered and processed; the backend rereads only their
// listings, and only when they are a minute old, so polling once a minute costs at most that.
const STOCK_REFRESH_MS = 60 * 1000
// The full listing is rebuilt in the background when the picker opens; look again after it.
const RELIST_SETTLE_MS = 15 * 1000
const STOCK_KINDS = ['chunks', 'packages', 'pieces', 'concentrates', 'molten']

export default {
  name: 'SkillOrePanel',
  props: {
    playerEntityId: { type: String, default: null },
    xpLeft: { type: Number, default: 0 },
    level: { type: Number, default: 1 },
    targetLevel: { type: Number, default: 100 },
    tier: { type: Number, default: null },
    trackedIds: { type: Array, default: () => [] },
    trackCrafts: { type: Boolean, default: false },
  },
  emits: ['update:tier', 'update:tracked', 'update:track-crafts'],
  setup(props, { emit }) {
    const data = ref(null)
    const error = ref('')
    const nowMs = ref(Date.now())
    const stock = ref(null)
    const stockError = ref('')
    const picking = ref(false)
    const showAllInventories = ref(false)
    let timer = null
    let stockTimer = null

    async function load() {
      const id = props.playerEntityId
      if (!id) {
        error.value = t('ore.noPlayer')
        return
      }
      data.value = data.value ?? readCachedOreData(id)
      try {
        const fresh = await fetchSmithingOre(id)
        if (id !== props.playerEntityId) return
        data.value = fresh
        error.value = ''
        writeCachedOreData(id, fresh)
      } catch (e) {
        error.value = t('ore.loadError', { error: e.message })
      }
      nowMs.value = Date.now()
    }
    async function loadStock({ relist = false } = {}) {
      const id = props.playerEntityId
      if (!id) return
      const key = `${id}:stock`
      stock.value = stock.value ?? readCachedOreData(key)
      try {
        const fresh = await fetchSmithingStock(id, { tracked: props.trackedIds.map(String), crafts: props.trackCrafts, relist })
        if (id !== props.playerEntityId) return
        stock.value = fresh
        stockError.value = ''
        writeCachedOreData(key, fresh)
      } catch (e) {
        stockError.value = t('ore.loadError', { error: e.message })
      }
    }
    // Nothing to keep fresh unless something is tracked or the picker is open, and nothing
    // while the browser tab is hidden.
    function pollStock() {
      if (document.visibilityState !== 'visible') return
      if (props.trackedIds.length || props.trackCrafts || picking.value) loadStock()
    }
    let settleTimer = null
    watch(picking, (open) => {
      clearTimeout(settleTimer)
      if (!open) return
      loadStock({ relist: true })
      settleTimer = setTimeout(() => { if (picking.value) loadStock() }, RELIST_SETTLE_MS)
    })
    watch(() => [props.trackedIds.join(','), props.trackCrafts], () => loadStock())
    onMounted(() => {
      load()
      loadStock()
      timer = setInterval(() => { if (document.visibilityState === 'visible') load() }, REFRESH_MS)
      stockTimer = setInterval(pollStock, STOCK_REFRESH_MS)
    })
    onUnmounted(() => {
      clearInterval(timer)
      clearInterval(stockTimer)
      clearTimeout(settleTimer)
    })
    watch(() => props.playerEntityId, () => {
      data.value = null
      stock.value = null
      load()
      loadStock()
    })

    const trackedSet = computed(() => new Set(props.trackedIds.map(String)))
    const tracking = computed(() => props.trackedIds.length > 0 || props.trackCrafts)
    const trackedTotals = computed(() => trackedStock(stock.value, props.trackedIds, { crafts: props.trackCrafts }))
    const trackLabel = computed(() => {
      if (!tracking.value) return t('ore.trackOff')
      const parts = []
      if (props.trackedIds.length) parts.push(t('ore.trackOn', { n: props.trackedIds.length }))
      if (props.trackCrafts) parts.push(t('ore.trackCrafts'))
      return parts.join(' · ')
    })
    const craftsSummary = computed(() => (stock.value?.crafts ?? [])
      .map((craft) => t('ore.craftLine', {
        tier: craft.tier,
        what: t(`ore.kind.${craft.kind}`, { n: formatNumber(craft.output) }),
        where: craft.building,
      }))
      .join(' · '))
    function toggleTracked(id) {
      const next = trackedSet.value.has(id)
        ? props.trackedIds.filter((x) => String(x) !== id)
        : [...props.trackedIds, id]
      emit('update:tracked', next)
    }
    const pickerGroups = computed(() => {
      const groups = new Map()
      for (const inv of stock.value?.inventories ?? []) {
        const hasOre = Object.keys(inv.counts ?? {}).length > 0
        // Wagons and caches are the player's own storage, so they are listed even while empty.
        const alwaysShown = inv.source === 'deployable' || trackedSet.value.has(inv.id)
        if (!showAllInventories.value && !hasOre && !alwaysShown) continue
        const key = `${inv.source}:${inv.source === 'claim' || inv.source === 'house' ? inv.place ?? '' : ''}`
        if (!groups.has(key)) {
          const title = inv.source === 'claim' || inv.source === 'house'
            ? inv.place
            : t(inv.source === 'deployable' ? 'ore.pickDeployables' : 'ore.pickPlayer')
          groups.set(key, { key, title, inventories: [] })
        }
        groups.get(key).inventories.push({
          ...inv,
          name: inv.source === 'player' && inv.place ? `${inv.name} · ${inv.place}` : inv.name,
        })
      }
      // Tracked inventories the listing no longer has (renamed, moved or gone) stay untickable.
      const listed = new Set((stock.value?.inventories ?? []).map((inv) => String(inv.id)))
      const gone = stock.value ? props.trackedIds.map(String).filter((id) => !listed.has(id)) : []
      if (gone.length) {
        groups.set('gone', {
          key: 'gone',
          title: t('ore.pickGone'),
          inventories: gone.map((id) => ({ id, name: id, missing: true })),
        })
      }
      return [...groups.values()]
    })
    function oreSummary(inv) {
      if (inv.missing) return t('ore.pickEmptyInventory')
      return Object.entries(inv.counts ?? {})
        .sort(([a], [b]) => Number(b) - Number(a))
        .map(([tier, counts]) => `T${tier} ${STOCK_KINDS
          .filter((kind) => counts[kind])
          .map((kind) => t(`ore.kind.${kind}`, { n: formatNumber(counts[kind]) }))
          .join(', ')}`)
        .join(' · ')
    }

    const rows = computed(() => (data.value?.setup?.tool && data.value.chains
      ? orePlan(data.value.chains, {
        xpLeft: props.xpLeft,
        setup: data.value.setup,
        level: props.level,
        market: data.value.market,
        stock: tracking.value ? trackedTotals.value : null,
      })
      : []))
    const selected = computed(() => rows.value.find((row) => row.tier === props.tier && !row.locked)
      ?? rows.value.find((row) => !row.locked)
      ?? null)
    const marketAge = computed(() => (data.value?.marketFetchedAt
      ? formatEta(Math.max(0, (nowMs.value - data.value.marketFetchedAt) / 1000))
      : ''))

    const formatMoney = (value) => formatXpShort(Math.round(value))
    function moneyText(row) {
      return row.money == null ? '—' : formatMoney(row.money)
    }
    function moneyLabel(row) {
      if (row.money == null) return t('ore.moneyNone', { ingot: row.ingotName })
      return t(row.priceFromBids ? 'ore.moneyLabelBids' : 'ore.moneyLabel', {
        n: formatNumber(row.ingots),
        price: formatNumber(row.ingotPrice),
      })
    }
    function stockTitle(row) {
      const parts = STOCK_KINDS
        .filter((kind) => row.stock?.[kind])
        .map((kind) => t(`ore.kind.${kind}`, { n: formatNumber(row.stock[kind]) }))
      if (row.stock?.effortLeft) parts.push(t('ore.effortLeft', { n: formatNumber(row.stock.effortLeft) }))
      return parts.length ? t('ore.haveTitle', { parts: parts.join(', ') }) : ''
    }
    function lockText(row) {
      return row.lockedByLevel ? t('ore.needLevel', { level: row.levelRequired }) : t('ore.needTool', { level: row.toolLevelRequired })
    }
    function rowTitle(row) {
      const lines = [t('ore.rowTitle', {
        ingot: row.ingotName,
        conc: row.concentratesPerIngot,
        pieces: row.piecesPerIngot,
        piece: row.pieceName,
        perPiece: formatNumber(Math.round(row.xpPerPiece)),
        perChunk: row.piecesPerChunk,
        chunk: row.chunkName,
      })]
      if (row.money != null) lines.push(moneyLabel(row))
      lines.push(row.instantSale.sold
        ? t('ore.instantLine', { sold: formatNumber(row.instantSale.sold), value: formatNumber(Math.round(row.instantSale.value)) })
        : t('ore.instantNone'))
      return lines.join('\n')
    }

    return {
      data, error, rows, selected, marketAge,
      stock, stockError, picking, showAllInventories, tracking, trackedSet, pickerGroups,
      toggleTracked, oreSummary, stockTitle, trackLabel, craftsSummary,
      moneyText, moneyLabel, lockText, rowTitle,
      formatMoney, formatNumber, formatPlayTime, formatXpCompact, t,
    }
  },
}
</script>

<style scoped>
.ore { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }

.ore-hero { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: 4px 20px; padding: 2px 2px 0; }
.ore-hero-figure { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.ore-hero-value { font-size: 34px; font-weight: 800; line-height: 1; color: var(--text); white-space: nowrap; font-variant-numeric: tabular-nums; }
.ore-hero-label, .ore-hero-side {
  min-width: 0;
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ore-hero-facts {
  display: grid;
  grid-template-columns: auto auto;
  gap: 1px 10px;
  margin: 0;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.ore-hero-facts dt { color: var(--muted); }
.ore-hero-facts dd { margin: 0; font-weight: 800; color: var(--text); text-align: right; }
.ore-hero-side { grid-column: 1 / -1; }

.ore-table-wrap { flex: 1; min-height: 0; overflow: auto; }
.ore-table { width: 100%; border-collapse: collapse; font-size: 11px; font-variant-numeric: tabular-nums; }
.ore-table th {
  position: sticky;
  top: 0;
  padding: 4px 6px;
  border-bottom: 1px solid var(--frame-soft);
  background: var(--panel);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  text-align: right;
  color: var(--muted);
  white-space: nowrap;
}
.ore-table td {
  padding: 4px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--frame-soft) 50%, transparent);
  text-align: right;
  color: var(--muted);
  white-space: nowrap;
}
.ore-table th.ore-main { color: var(--text-soft); }
.ore-table td.ore-main { font-size: 14px; font-weight: 800; color: var(--text); }
.ore-table .ore-gap { padding-left: 18px; }
.ore-have { display: block; font-size: 10px; font-weight: 600; color: var(--muted); }
.ore-table .ore-col-name { text-align: left; }
.ore-table tbody tr { cursor: pointer; }
.ore-table tbody tr:hover td { background: rgba(233, 223, 196, 0.04); }
.ore-tier { display: inline-block; min-width: 22px; font-weight: 800; color: var(--muted); }
.ore-name { font-weight: 700; color: var(--text-soft); }
.ore-lock { margin-left: 6px; font-size: 10px; color: var(--muted); }
.ore-row--selected td { background: rgba(233, 223, 196, 0.06); }
.ore-row--selected td:first-child { box-shadow: inset 2px 0 0 var(--text-soft); }
.ore-row--locked { cursor: default; }
.ore-row--locked td { opacity: 0.45; }

.ore-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; font-size: 11px; color: var(--muted); }
.ore-track-btn {
  padding: 2px 8px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: transparent;
  color: var(--text-soft);
  font: inherit;
  cursor: pointer;
}
.ore-track-btn:hover { border-color: var(--frame); }
.ore-track-btn--on { color: var(--text); border-color: var(--text-soft); }

.ore-picker { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 8px; }
.ore-picker-head { display: flex; align-items: center; gap: 12px; }
.ore-picker-title { font-size: 12px; font-weight: 800; color: var(--text-soft); }
.ore-picker-all { display: inline-flex; align-items: center; gap: 4px; margin-left: auto; font-size: 11px; color: var(--muted); cursor: pointer; }
.ore-picker-done { padding: 3px 10px; font-size: 11px; }
.ore-picker-list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 8px; }
.ore-picker-place {
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
}
.ore-picker-row { display: flex; align-items: baseline; gap: 6px; padding: 2px 0; font-size: 12px; cursor: pointer; }
.ore-picker-row input { align-self: center; }
.ore-picker-crafts .ore-picker-name { font-weight: 700; }
.ore-picker-name { color: var(--text); white-space: nowrap; }
.ore-picker-ore { min-width: 0; font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
