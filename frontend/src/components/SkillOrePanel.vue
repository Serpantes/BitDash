<template>
  <div class="ore">
    <div v-if="error && !data" class="bc-error">{{ error }}</div>
    <div v-else-if="data && !data.setup?.tool" class="bc-muted-note">{{ t('ore.noHammer') }}</div>
    <template v-else-if="data">
      <div v-if="xpLeft <= 0" class="bc-muted-note">{{ t('ore.reached', { level: targetLevel }) }}</div>
      <template v-else>
        <div v-if="selected" class="ore-hero" :title="rowTitle(selected)">
          <div class="ore-hero-figure">
            <span class="ore-hero-value">{{ selected.chunks == null ? '—' : formatNumber(selected.chunks) }}</span>
            <span class="ore-hero-label">{{ selected.chunkName }}</span>
          </div>
          <div class="ore-hero-figure">
            <span class="ore-hero-value">{{ moneyText(selected) }}</span>
            <span class="ore-hero-label">{{ moneyLabel(selected) }}</span>
          </div>
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
                <th class="ore-gap" :title="t('ore.colPriceTitle')">{{ t('ore.colPrice') }}</th>
                <th :title="t('ore.colInstantTitle')">{{ t('ore.colInstant') }}</th>
                <th>{{ t('ore.colIngots') }}</th>
                <th>{{ t('ore.colFlux') }}</th>
                <th :title="t('ore.colSwingTitle')">{{ t('ore.colSwing') }}</th>
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
                <td class="ore-main">{{ row.chunks == null ? '—' : formatNumber(row.chunks) }}</td>
                <td class="ore-main">{{ moneyText(row) }}</td>
                <td class="ore-gap">{{ row.ingotPrice == null ? '—' : formatNumber(row.ingotPrice) }}</td>
                <td>{{ row.instantSale.sold ? formatMoney(row.instantSale.value) : '—' }}</td>
                <td>{{ formatNumber(row.ingots) }}</td>
                <td>{{ row.flux ? formatNumber(row.flux) : '—' }}</td>
                <td>{{ formatNumber(row.swingXp) }}</td>
                <td>{{ formatPlayTime(row.seconds) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <div class="ore-foot">
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
import { fetchSmithingOre } from '../api'
import { formatNumber, t } from '../i18n'
import { formatXpCompact, formatXpShort } from '../skillProgress'
import { formatEta, formatPlayTime } from '../xpHistory'
import { orePlan, readCachedOreData, writeCachedOreData } from '../smithingOre'

// The backend keeps setups and order books fresh within minutes; polling slower is enough.
const REFRESH_MS = 5 * 60 * 1000

export default {
  name: 'SkillOrePanel',
  props: {
    playerEntityId: { type: String, default: null },
    xpLeft: { type: Number, default: 0 },
    level: { type: Number, default: 1 },
    targetLevel: { type: Number, default: 100 },
    tier: { type: Number, default: null },
  },
  emits: ['update:tier'],
  setup(props) {
    const data = ref(null)
    const error = ref('')
    const nowMs = ref(Date.now())
    let timer = null

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
    onMounted(() => {
      load()
      timer = setInterval(load, REFRESH_MS)
    })
    onUnmounted(() => clearInterval(timer))
    watch(() => props.playerEntityId, () => {
      data.value = null
      load()
    })

    const rows = computed(() => (data.value?.setup?.tool && data.value.chains
      ? orePlan(data.value.chains, { xpLeft: props.xpLeft, setup: data.value.setup, level: props.level, market: data.value.market })
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
      moneyText, moneyLabel, lockText, rowTitle,
      formatMoney, formatNumber, formatPlayTime, formatXpCompact, t,
    }
  },
}
</script>

<style scoped>
.ore { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }

.ore-hero { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px 20px; padding: 2px 2px 0; }
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

.ore-foot { display: flex; flex-wrap: wrap; gap: 4px 14px; font-size: 11px; color: var(--muted); }
</style>
