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
            <button type="button" class="ore-tier-btn" :aria-expanded="tableOpen" @click="tableOpen = !tableOpen">
              T{{ selected.tier }} {{ selected.ingotName.replace(/ Ingot$/, '') }} {{ tableOpen ? '▴' : '▾' }}
            </button>
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
        </div>

        <div v-if="tableOpen || !selected" class="ore-table-wrap">
          <table class="ore-table">
            <thead>
              <tr>
                <th class="ore-col-name">{{ t('ore.colTier') }}</th>
                <th class="ore-main" :title="t('ore.colChunksTitle')">{{ t('ore.colChunks') }}</th>
                <th :title="t('ore.colPiecesTitle')">{{ t('ore.colPieces') }}</th>
                <th>{{ t('ore.colIngots') }}</th>
                <th class="ore-main" :title="t('ore.colMoneyTitle')">{{ t('ore.colMoney') }}</th>
                <th :title="t('ore.colTimeTitle')">{{ t('ore.colTime') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rows"
                :key="row.tier"
                :class="{ 'ore-row--selected': row.tier === selected?.tier, 'ore-row--locked': row.locked }"
                :title="rowTitle(row)"
                @click="pickTier(row)"
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
                <td :title="stockTitle(row)">
                  {{ formatNumber(row.pieces) }}
                  <span v-if="row.havePieces" class="ore-have">{{ t('ore.haveOf', { have: formatNumber(row.havePieces), total: formatNumber(row.piecesTotal) }) }}</span>
                </td>
                <td>{{ formatNumber(row.ingots) }}</td>
                <td class="ore-main">{{ moneyText(row) }}</td>
                <td>{{ formatPlayTime(row.seconds) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="ore-progress">
          <div class="ore-progress-head">
            <span class="ore-progress-title">{{ t('ore.gathered') }}</span>
            <span class="ore-progress-pct">{{ formatSkillPct(gatheredPct) }}</span>
          </div>
          <div class="bc-bar ore-progress-bar" role="meter" :aria-valuenow="selected.haveChunks" aria-valuemin="0" :aria-valuemax="selected.chunksTotal">
            <div class="bc-bar-track"><div class="bc-bar-fill" :style="{ width: `${gatheredPct}%` }"></div></div>
            <span class="bc-bar-text">{{ formatNumber(selected.haveChunks) }} / {{ formatNumber(selected.chunksTotal) }}</span>
          </div>
          <div v-if="tracking" class="ore-speed" :title="t('ore.speedTitle')">
            <span class="ore-speed-label">{{ t('ore.speed') }}</span>
            <template v-if="speed">
              <span class="ore-speed-value">{{ t('ore.speedValue', { n: formatNumber(Math.round(speed.perHour)) }) }}</span>
              <span v-if="speed.perHour >= 1" class="ore-speed-eta">{{ t('ore.speedEta', { time: formatPlayTime((selected.chunks / speed.perHour) * 3600) }) }}</span>
              <span v-else class="ore-speed-eta">{{ t('ore.speedIdle') }}</span>
            </template>
            <span v-else class="ore-speed-eta">{{ t('ore.speedWarmup') }}</span>
          </div>

          <template v-if="places.length">
            <div class="ore-places-title">{{ t('ore.wherePlaces') }}</div>
            <ul class="ore-places">
              <li v-for="place in places" :key="place.key" class="ore-place" :title="place.title">
                <span class="ore-place-name">{{ place.name }}</span>
                <span class="ore-place-track"><span class="ore-place-fill" :style="{ width: `${place.share}%` }"></span></span>
                <span class="ore-place-value">{{ formatNumber(Math.round(place.chunks)) }}</span>
              </li>
            </ul>
          </template>
          <div v-else class="ore-progress-empty">
            {{ tracking ? t('ore.nothingForTier', { tier: selected.tier }) : t('ore.nothingTracked') }}
            <button v-if="!tracking" type="button" class="ore-track-btn" @click="picking = true">{{ t('ore.trackOff') }}</button>
          </div>
        </div>
      </template>

      <div class="ore-foot">
        <button type="button" class="ore-track-btn" :class="{ 'ore-track-btn--on': tracking }" @click="picking = !picking">
          {{ trackLabel }}
        </button>
        <span>×{{ formatNumber(data.setup.xpRate, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }} XP</span>
      </div>
    </template>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { fetchSmithingOre, fetchSmithingStock } from '../api'
import { formatNumber, t } from '../i18n'
import { formatSkillPct, formatXpShort } from '../skillProgress'
import { formatPlayTime } from '../xpHistory'
import {
  appendGatherSample,
  gatherRate,
  orePlan,
  rawChunks,
  readCachedOreData,
  stockAsChunks,
  trackedStock,
  writeCachedOreData,
} from '../smithingOre'

// The backend keeps setups and order books fresh within minutes; polling slower is enough.
const REFRESH_MS = 5 * 60 * 1000
// Tracked inventories change as ore is gathered and processed; the backend rereads only their
// listings and the crafts, and only once they are ten seconds old.
const STOCK_REFRESH_MS = 10 * 1000
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
    const stock = ref(null)
    const stockError = ref('')
    const picking = ref(false)
    // The table is for choosing a tier; once one is chosen, only its numbers stay on screen.
    const tableOpen = ref(props.tier == null)
    function pickTier(row) {
      if (row.locked) return
      emit('update:tier', row.tier)
      tableOpen.value = false
    }
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
        recordGather(id)
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
      gatherHistory.value = readCachedOreData(`${props.playerEntityId}:gather`)
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
      gatherHistory.value = readCachedOreData(`${props.playerEntityId}:gather`)
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

    // Gathering speed: raw ore per tier in the tracked places, sampled on every refresh.
    const gatherHistory = ref(null)
    const nowSec = ref(Math.floor(Date.now() / 1000))
    function recordGather(id) {
      const chains = data.value?.chains
      if (!chains || !tracking.value) return
      const totals = trackedStock(stock.value, props.trackedIds, { crafts: props.trackCrafts })
      const byTier = {}
      for (const chain of chains) byTier[chain.tier] = rawChunks(chain, totals[String(chain.tier)])
      const key = `${id}:gather`
      const signature = `${[...props.trackedIds].map(String).sort().join(',')}|${props.trackCrafts ? 1 : 0}`
      nowSec.value = Math.floor(Date.now() / 1000)
      gatherHistory.value = appendGatherSample(gatherHistory.value, { t: nowSec.value, signature, byTier })
      writeCachedOreData(key, gatherHistory.value)
    }
    const speed = computed(() => (selected.value
      ? gatherRate(gatherHistory.value, selected.value.tier, nowSec.value)
      : null))
    const gatheredPct = computed(() => (selected.value?.chunksTotal
      ? Math.min(100, (selected.value.haveChunks / selected.value.chunksTotal) * 100)
      : 0))
    // Where the chosen tier's gathered ore sits, as chunks, largest first.
    const places = computed(() => {
      const row = selected.value
      const chain = data.value?.chains?.find((c) => c.tier === row?.tier)
      if (!row || !chain) return []
      const tier = String(row.tier)
      const kindsOf = (counts) => STOCK_KINDS
        .filter((kind) => counts[kind])
        .map((kind) => t(`ore.kind.${kind}`, { n: formatNumber(counts[kind]) }))
        .join(', ')
      const list = []
      for (const inv of stock.value?.inventories ?? []) {
        const counts = inv.counts?.[tier]
        if (!counts || !trackedSet.value.has(String(inv.id))) continue
        list.push({
          key: inv.id,
          name: inv.place && inv.source !== 'deployable' ? `${inv.name} · ${inv.place}` : inv.name,
          chunks: stockAsChunks(chain, data.value.setup, counts),
          title: kindsOf(counts),
        })
      }
      if (props.trackCrafts) {
        const counts = trackedStock({ crafts: stock.value?.crafts }, [], { crafts: true })[tier]
        if (counts) {
          list.push({
            key: 'crafts',
            name: t('ore.pickCrafts'),
            chunks: stockAsChunks(chain, data.value.setup, counts),
            title: [kindsOf(counts), counts.effortLeft ? t('ore.effortLeft', { n: formatNumber(counts.effortLeft) }) : ''].filter(Boolean).join(', '),
          })
        }
      }
      const shown = list.filter((place) => place.chunks >= 0.5).sort((a, b) => b.chunks - a.chunks)
      const max = Math.max(...shown.map((place) => place.chunks), 1)
      return shown.map((place) => ({ ...place, share: (place.chunks / max) * 100 }))
    })
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
      data, error, rows, selected, tableOpen, pickTier, gatheredPct, places, formatSkillPct, speed,
      stock, stockError, picking, showAllInventories, tracking, trackedSet, pickerGroups,
      toggleTracked, oreSummary, stockTitle, trackLabel, craftsSummary,
      moneyText, moneyLabel, lockText, rowTitle,
      formatMoney, formatNumber, formatPlayTime, t,
    }
  },
}
</script>

<style scoped>
.ore { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }

.ore-hero { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: 4px 20px; padding: 2px 2px 0; }
.ore-hero-figure { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.ore-hero-value { font-size: 34px; font-weight: 800; line-height: 1; color: var(--text); white-space: nowrap; font-variant-numeric: tabular-nums; }
.ore-hero-label {
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
.ore-tier-btn {
  align-self: flex-start;
  margin-bottom: 4px;
  padding: 1px 8px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: transparent;
  color: var(--text-soft);
  font: inherit;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
}
.ore-tier-btn:hover { border-color: var(--frame); color: var(--text); }
.ore-hero-facts dd { margin: 0; font-weight: 800; color: var(--text); text-align: right; }

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

.ore-progress { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 8px; overflow: auto; }
.ore-progress-head { display: flex; justify-content: space-between; align-items: baseline; }
.ore-progress-title, .ore-places-title {
  font-family: var(--font-title);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--text-soft);
}
.ore-progress-pct { font-size: 15px; font-weight: 800; color: var(--text); }
.ore-progress-bar { height: 24px; }
.ore-speed { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; font-size: 12px; color: var(--muted); }
.ore-speed-label { color: var(--text-soft); }
.ore-speed-value { font-size: 14px; font-weight: 800; color: var(--text); }
.ore-places-title { margin-top: 4px; font-size: 10px; color: var(--muted); }
.ore-places { display: flex; flex-direction: column; gap: 5px; margin: 0; padding: 0; list-style: none; }
.ore-place { display: grid; grid-template-columns: minmax(0, 11em) minmax(0, 1fr) auto; align-items: center; gap: 10px; font-size: 12px; }
.ore-place-name { color: var(--text-soft); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ore-place-track { height: 8px; }
.ore-place-fill { display: block; height: 100%; min-width: 2px; border-radius: 0 4px 4px 0; background: var(--accent); }
.ore-place-value { min-width: 4ch; text-align: right; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.ore-progress-empty { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }

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
