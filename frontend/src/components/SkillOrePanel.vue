<template>
  <div class="ore">
    <div v-if="error && !data" class="bc-error">{{ error }}</div>
    <div v-else-if="data && !data.setup?.tool" class="bc-muted-note">{{ t('ore.noHammer') }}</div>
    <template v-else-if="data">
      <div class="ore-kpis">
        <div class="ore-kpi" :title="t('ore.rateTitle')">
          <span class="ore-kpi-label">{{ t('ore.rate') }}</span>
          <span class="ore-kpi-value">×{{ formatNumber(data.setup.xpRate, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }}</span>
          <span class="ore-kpi-sub">{{ t('ore.rateSub', { pct: formatNumber(Math.round(data.setup.xpRate * 100)) }) }}</span>
        </div>
        <div class="ore-kpi" :title="t('ore.progressTitle')">
          <span class="ore-kpi-label">{{ t('ore.progress') }}</span>
          <span class="ore-kpi-value">{{ formatNumber(data.setup.progressPerSwing) }}</span>
          <span class="ore-kpi-sub">{{ t('ore.progressSub', { tool: data.setup.tool.name, power: data.setup.tool.power, skill: data.setup.skillPower }) }}</span>
        </div>
        <div class="ore-kpi">
          <span class="ore-kpi-label">{{ t('ore.left', { level: targetLevel }) }}</span>
          <span class="ore-kpi-value">{{ formatXpCompact(xpLeft) }}<small> XP</small></span>
          <span class="ore-kpi-sub">{{ t('ore.leftSub') }}</span>
        </div>
      </div>

      <div v-if="xpLeft <= 0" class="bc-muted-note">{{ t('ore.reached', { level: targetLevel }) }}</div>
      <div v-else class="ore-table-wrap">
        <table class="ore-table">
          <thead>
            <tr>
              <th class="ore-col-name">{{ t('ore.colIngot') }}</th>
              <th class="ore-key">{{ t('ore.colIngots') }}</th>
              <th class="ore-key" :title="t('ore.colChunksTitle')">{{ t('ore.colChunks') }}</th>
              <th class="ore-key">{{ t('ore.colFlux') }}</th>
              <th>{{ t('ore.colPieces') }}</th>
              <th :title="t('ore.colSwingTitle')">{{ t('ore.colSwing') }}</th>
              <th :title="t('ore.colPerIngotTitle')">{{ t('ore.colPerIngot') }}</th>
              <th :title="t('ore.colTimeTitle')">{{ t('ore.colTime') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in rows"
              :key="row.tier"
              :class="{ 'ore-row--best': row.best, 'ore-row--locked': row.locked }"
              :title="rowTitle(row)"
            >
              <td class="ore-col-name">
                <span class="ore-tier">T{{ row.tier }}</span>
                <span class="ore-name">{{ row.ingotName.replace(/ Ingot$/, '') }}</span>
                <span v-if="row.locked" class="ore-lock">{{ lockText(row) }}</span>
                <span v-else-if="row.best" class="ore-best">{{ t('ore.best') }}</span>
              </td>
              <td class="ore-key">{{ formatNumber(row.ingots) }}</td>
              <td class="ore-key">{{ row.chunks == null ? '—' : formatNumber(row.chunks) }}</td>
              <td class="ore-key">{{ row.flux ? formatNumber(row.flux) : '—' }}</td>
              <td>{{ formatNumber(row.pieces) }}</td>
              <td>{{ formatNumber(row.swingXp) }}</td>
              <td>{{ formatNumber(Math.round(row.xpPerIngot)) }}</td>
              <td>{{ formatPlayTime(row.seconds) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="ore-foot">{{ t('ore.foot') }}</div>
    </template>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { fetchSmithingOre } from '../api'
import { formatNumber, t } from '../i18n'
import { formatXpCompact } from '../skillProgress'
import { formatPlayTime } from '../xpHistory'
import { orePlan, readCachedOreData, writeCachedOreData } from '../smithingOre'

// The backend keeps setups fresh within a few minutes; polling slower than that is enough.
const REFRESH_MS = 5 * 60 * 1000

export default {
  name: 'SkillOrePanel',
  props: {
    playerEntityId: { type: String, default: null },
    xpLeft: { type: Number, default: 0 },
    level: { type: Number, default: 1 },
    targetLevel: { type: Number, default: 100 },
  },
  setup(props) {
    const data = ref(null)
    const error = ref('')
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
      ? orePlan(data.value.chains, { xpLeft: props.xpLeft, setup: data.value.setup, level: props.level })
      : []))

    function lockText(row) {
      return row.lockedByLevel ? t('ore.needLevel', { level: row.levelRequired }) : t('ore.needTool', { level: row.toolLevelRequired })
    }
    function rowTitle(row) {
      return t('ore.rowTitle', {
        ingot: row.ingotName,
        conc: row.concentratesPerIngot,
        pieces: row.piecesPerIngot,
        piece: row.pieceName,
        flux: row.fluxPerIngot,
        perPiece: formatNumber(Math.round(row.xpPerPiece)),
        perChunk: row.piecesPerChunk,
        chunk: row.chunkName,
      })
    }

    return { data, error, rows, lockText, rowTitle, formatNumber, formatPlayTime, formatXpCompact, t }
  },
}
</script>

<style scoped>
.ore { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }
.ore-kpis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
.ore-kpi {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  padding: 6px 9px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.35);
}
.ore-kpi-label {
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-soft);
  white-space: nowrap;
}
.ore-kpi-value { font-size: 18px; font-weight: 800; line-height: 1.15; color: var(--text); white-space: nowrap; font-variant-numeric: tabular-nums; }
.ore-kpi-value small { font-size: 11px; font-weight: 700; color: var(--muted); }
.ore-kpi-sub { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.ore-table-wrap { flex: 1; min-height: 0; overflow: auto; }
.ore-table { width: 100%; border-collapse: collapse; font-size: 12px; font-variant-numeric: tabular-nums; }
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
  color: var(--text-soft);
  white-space: nowrap;
}
.ore-table td {
  padding: 4px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--frame-soft) 50%, transparent);
  text-align: right;
  color: var(--text-soft);
  white-space: nowrap;
}
.ore-table .ore-col-name { text-align: left; }
.ore-tier { display: inline-block; min-width: 22px; font-weight: 800; color: var(--muted); }
.ore-name { font-weight: 700; color: var(--text); }
.ore-table th.ore-key { color: var(--gold); }
.ore-table td.ore-key {
  font-size: 13px;
  font-weight: 800;
  color: var(--text);
  background: color-mix(in srgb, var(--gold) 9%, transparent);
}
.ore-table th.ore-key + th:not(.ore-key), .ore-table td.ore-key + td:not(.ore-key) { border-left: 1px solid var(--frame-soft); }
.ore-best, .ore-lock { margin-left: 6px; font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.06em; }
.ore-best { color: var(--success); }
.ore-lock { color: var(--muted); }
.ore-row--best td { background: color-mix(in srgb, var(--success) 10%, transparent); }
.ore-row--best .ore-tier { color: var(--success); }
.ore-row--locked td { opacity: 0.45; }
.ore-foot { font-size: 11px; color: var(--muted); }
</style>
