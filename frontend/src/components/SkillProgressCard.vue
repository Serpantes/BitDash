<template>
  <div v-if="!progress" class="bc-muted-note">{{ t('common.noData') }}</div>
  <div v-else class="skill-card" :class="`skill-card--${size}`">
    <div class="skill-summary">
      <div class="skill-hero">
        <div class="skill-level-box">
          <span class="skill-caption">{{ t('skillCard.level') }}</span>
          <span class="skill-level">{{ progress.level }}<span class="skill-level-cap">/ {{ progress.targetLevel }}</span></span>
        </div>
        <button
          type="button"
          class="skill-rank"
          :class="{ 'skill-rank--active': isActiveScope }"
          :title="rankTitle"
          :aria-pressed="isActiveScope"
          @click="$emit('update-view', { skillRankScope: isActiveScope ? 'all' : 'active' })"
        >
          <span
            v-for="face in rankFaces"
            :key="face.scope"
            class="skill-rank-face"
            :class="{ 'skill-rank-face--hidden': face.scope !== rankFace.scope }"
            :aria-hidden="face.scope !== rankFace.scope"
          >
            <span class="skill-caption skill-rank-caption">{{ face.caption }}</span>
            <span class="skill-rank-value">{{ face.value }}</span>
            <span class="skill-rank-meta">{{ face.meta }}</span>
            <span v-if="face.next" class="skill-rank-next" :title="face.next.title">{{ face.next.text }}</span>
          </span>
        </button>
      </div>

      <div class="skill-meter">
        <div class="skill-meter-head">
          <span class="skill-caption">{{ progress.isMax ? t('skillCard.maxLevel') : t('skillCard.toLevel', { level: progress.level + 1 }) }}</span>
          <span class="skill-meter-value">{{ formatSkillPct(progress.levelPct) }}</span>
        </div>
        <div class="bc-bar skill-bar">
          <div class="bc-bar-track"><div class="bc-bar-fill" :style="{ width: `${progress.levelPct}%` }"></div></div>
          <span v-if="!progress.isMax" class="bc-bar-text">{{ formatXpCompact(progress.levelXp) }} / {{ formatXpCompact(progress.levelSpan) }}</span>
        </div>
        <div v-if="!progress.isMax" class="skill-meter-foot" :title="etaTitle">
          <span>{{ t('skillCard.xpLeft', { xp: formatXpCompact(progress.toNext) }) }}</span>
          <span v-if="etaNext">{{ t('skillCard.ofPlay', { time: etaNext }) }}</span>
        </div>
      </div>

      <div class="skill-meter" :class="{ 'skill-meter--done': progress.reachedTarget }">
        <div class="skill-meter-head">
          <span class="skill-caption">
            {{ progress.reachedTarget ? t('skillCard.reached', { level: progress.targetLevel }) : t('skillCard.toLevel', { level: progress.targetLevel }) }}
          </span>
          <span class="skill-meter-value">{{ formatSkillPct(progress.targetPct) }}</span>
        </div>
        <div class="bc-bar skill-bar">
          <div class="bc-bar-track">
            <div class="bc-bar-fill" :style="{ width: `${progress.targetPct}%` }"></div>
            <span
              v-for="tick in pathTicks"
              :key="tick.level"
              class="skill-tick"
              :style="{ left: `${tick.pct}%` }"
            ></span>
          </div>
          <span class="bc-bar-text">{{ formatXpCompact(progress.xp) }} / {{ formatXpCompact(progress.targetXp) }}</span>
        </div>
        <div class="skill-tick-labels" aria-hidden="true">
          <span
            v-for="tick in pathTicks"
            :key="tick.level"
            class="skill-tick-label"
            :style="{ left: `${tick.pct}%` }"
          >{{ tick.level }}</span>
        </div>
        <div class="skill-meter-foot" :title="etaTitle">
          <span v-if="!progress.reachedTarget">{{ t('skillCard.xpLeft', { xp: formatXpCompact(progress.toTarget) }) }}</span>
          <span v-else>{{ t('skillCard.overTarget', { xp: formatXpCompact(progress.xp - progress.targetXp) }) }}</span>
          <span v-if="etaTarget">{{ t('skillCard.ofPlay', { time: etaTarget }) }}</span>
        </div>
        <div v-if="targetForecast" class="skill-forecast" :title="targetForecast.title">
          <div class="skill-forecast-head">
            <span class="skill-caption">{{ t('skillCard.plan', { level: progress.targetLevel }) }}</span>
            <span class="skill-forecast-date">{{ targetForecast.date }}</span>
          </div>
          <div class="skill-forecast-sub">{{ targetForecast.sub }}</div>
        </div>
      </div>
    </div>

    <div v-if="size === 'large'" class="skill-chart">
      <div class="skill-chart-head">
        <div class="bc-seg">
          <button type="button" class="bc-seg-btn" :class="{ active: activeTab === 'pace' }" @click="$emit('update-view', { skillChartTab: 'pace' })">{{ t('skillCard.activityTab') }}</button>
          <button type="button" class="bc-seg-btn" :class="{ active: activeTab === 'curve' }" @click="$emit('update-view', { skillChartTab: 'curve' })">{{ t('skillCard.curveTab') }}</button>
        </div>
        <span v-if="activeTab === 'curve'" class="skill-chart-sub">{{ t('skillCard.curveSub') }}</span>
      </div>
      <SkillActivityPanel
        v-if="activeTab === 'pace'"
        :points="history"
        :level-table="levelTable"
        :now="nowSec"
        :period="pacePeriod"
        :tracked-since="trackedSince"
        @update:period="$emit('update-view', { skillPacePeriod: $event })"
      />
      <SkillCurveChart v-else :level-table="levelTable" :progress="progress" />
    </div>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  formatSkillPct,
  formatTopPct,
  formatXpCompact,
  formatXpShort,
  skillProgress,
  targetPathTicks,
} from '../skillProgress'
import { activePace, forecastEta, formatDate, formatEta, formatPlayTime, formatUntil } from '../xpHistory'
import { formatNumber, t } from '../i18n'
import SkillCurveChart from './SkillCurveChart.vue'
import SkillActivityPanel from './SkillActivityPanel.vue'

const NOW_TICK_MS = 15000

export default {
  name: 'SkillProgressCard',
  components: { SkillActivityPanel, SkillCurveChart },
  props: {
    levelTable: { type: Array, default: null },
    xp: { type: Number, default: 0 },
    rank: { type: Number, default: null },
    totalPlayers: { type: Number, default: null },
    above: { type: Object, default: null },
    rankScope: { type: String, default: 'all' },
    activeRanking: { type: Object, default: null },
    activeDays: { type: Number, default: 30 },
    skillId: { type: [String, Number], default: null },
    size: { type: String, default: 'small' },
    history: { type: Array, default: null },
    trackedSince: { type: Number, default: null },
    chartTab: { type: String, default: 'pace' },
    pacePeriod: { type: String, default: '7d' },
  },
  emits: ['update-view'],
  setup(props) {
    const nowSec = ref(Math.floor(Date.now() / 1000))
    let nowTimer = null
    onMounted(() => { nowTimer = setInterval(() => { nowSec.value = Math.floor(Date.now() / 1000) }, NOW_TICK_MS) })
    onUnmounted(() => clearInterval(nowTimer))

    const progress = computed(() => skillProgress(props.levelTable, props.xp))
    const pathTicks = computed(() => (progress.value ? targetPathTicks(props.levelTable, progress.value.targetLevel) : []))
    const activeTab = computed(() => (props.chartTab === 'curve' ? 'curve' : 'pace'))
    const playPace = computed(() => activePace(props.history, nowSec.value, 7 * 86400) ?? activePace(props.history, nowSec.value, null))
    const etaFor = (remaining) => (playPace.value?.perHour > 0 ? formatPlayTime((remaining / playPace.value.perHour) * 3600) : '')
    const etaNext = computed(() => (progress.value && !progress.value.isMax ? etaFor(progress.value.toNext) : ''))
    const etaTarget = computed(() => (progress.value && !progress.value.reachedTarget ? etaFor(progress.value.toTarget) : ''))
    const etaTitle = computed(() => (playPace.value
      ? t('skillCard.etaTitle', { pace: formatXpCompact(playPace.value.perHour) })
      : ''))
    const targetForecast = computed(() => {
      if (!progress.value || progress.value.reachedTarget) return null
      const level = progress.value.targetLevel
      const eta = forecastEta(props.history, nowSec.value, progress.value.toTarget)
      if (!eta) {
        return {
          date: '—',
          sub: t('skillCard.littleHistory'),
          title: t('skillCard.littleHistoryTitle', { level }),
        }
      }
      const perDay = t('skillCard.perDay', { xp: formatXpShort(eta.perDay) })
      const basis = t('skillCard.basis', { xp: formatXpCompact(eta.perDay), span: formatEta(eta.basedOnSec) })
      if (eta.at == null) {
        return {
          date: t('skillCard.over100Years'),
          sub: perDay,
          title: t('skillCard.beyondCentury', { xp: formatXpCompact(progress.value.toTarget), level, basis }),
        }
      }
      return {
        date: formatDate(eta.at),
        sub: `${formatUntil(eta.sec)} · ${perDay}`,
        title: t('skillCard.forecastTitle', { xp: formatXpCompact(progress.value.toTarget), level, basis }),
      }
    })
    function nextPlaceFor(rank, above) {
      if (!above || !rank) return null
      const who = `#${above.rank}${above.username ? ` — ${above.username}` : ''}, ${formatCount(above.xp)} XP`
      if (above.rank > rank) {
        const lead = props.xp - above.xp
        return lead > 0 ? { text: t('skillCard.leadOver', { rank: above.rank, xp: formatXpShort(lead) }), title: who } : null
      }
      const gap = above.xp - props.xp
      if (gap <= 0) return { text: t('skillCard.alreadyPassed', { rank: above.rank }), title: t('skillCard.alreadyPassedTitle', { who }) }
      const eta = etaFor(gap)
      return {
        text: t('skillCard.gapTo', { rank: above.rank, xp: formatXpShort(gap) }),
        title: eta ? t('skillCard.gapTitle', { who, eta }) : who,
      }
    }
    function faceFor(scope) {
      const active = scope === 'active'
      const src = active
        ? (props.activeRanking ?? { loading: true })
        : { rank: props.rank, totalPlayers: props.totalPlayers, above: props.above }
      const rank = src.rank ?? null
      const total = src.totalPlayers ?? null
      let meta = t('skillCard.notRanked')
      if (rank) meta = total ? t('skillCard.ofTotal', { total: formatCount(total), top: formatTopPct((rank / total) * 100) }) : ''
      else if (active && src.loading) meta = t('skillCard.loading')
      else if (active && props.xp > 0) meta = t('skillCard.inactive', { days: props.activeDays })
      return {
        scope,
        rank,
        total,
        caption: t(active ? 'skillCard.captionActive' : 'skillCard.captionAll'),
        value: rank ? `#${formatCount(rank)}` : '—',
        meta,
        next: nextPlaceFor(rank, src.above),
      }
    }
    const rankFaces = computed(() => [faceFor('all'), faceFor('active')])
    const isActiveScope = computed(() => props.rankScope === 'active')
    const rankFace = computed(() => rankFaces.value[isActiveScope.value ? 1 : 0])
    const rankTitle = computed(() => {
      const { rank, total } = rankFace.value
      const who = isActiveScope.value
        ? t('skillCard.whoActive', { days: props.activeDays })
        : t('skillCard.whoAll')
      const place = rank
        ? t('skillCard.placeAmong', { rank: formatCount(rank), total: total ? formatCount(total) : '?', who })
        : t('skillCard.notRankedAmong', { who })
      const toggle = isActiveScope.value ? t('skillCard.toggleToAll') : t('skillCard.toggleToActive', { days: props.activeDays })
      return `${place}. ${toggle}`
    })

    function formatCount(n) {
      return formatNumber(n ?? 0)
    }

    return {
      nowSec, progress, pathTicks, activeTab, etaNext, etaTarget, etaTitle, targetForecast, rankTitle,
      isActiveScope, rankFaces, rankFace,
      formatCount, formatSkillPct, formatTopPct, formatXpCompact,
    }
  },
}
</script>

<style scoped>
.skill-card { flex: 1; min-height: 0; display: flex; gap: 20px; }
.skill-card--small { flex-direction: column; justify-content: center; }
.skill-summary { display: flex; flex-direction: column; gap: 18px; justify-content: center; min-width: 0; }
.skill-card--small .skill-summary { flex: 1; }
.skill-card--large .skill-summary { flex: 0 0 auto; min-width: 280px; gap: 16px; }

.skill-caption {
  font-family: var(--font-title);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--text-soft);
  white-space: nowrap;
}
.skill-hero { display: flex; justify-content: space-between; align-items: stretch; gap: 12px; }
.skill-level-box { display: flex; flex-direction: column; justify-content: center; gap: 2px; }
.skill-level { font-size: 54px; font-weight: 800; line-height: 1; color: var(--text); font-variant-numeric: tabular-nums; white-space: nowrap; }
.skill-level-cap { margin-left: 6px; font-size: 18px; font-weight: 700; color: var(--muted); }
.skill-rank {
  display: grid;
  align-content: center;
  min-width: 118px;
  padding: 8px 12px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.35);
  color: inherit;
  font: inherit;
  text-align: right;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.skill-rank:hover { border-color: var(--frame); background: rgba(233, 223, 196, 0.05); }
.skill-rank--active { border-color: color-mix(in srgb, var(--success) 45%, transparent); }
.skill-rank--active:hover { border-color: color-mix(in srgb, var(--success) 75%, transparent); }
.skill-rank-face {
  grid-area: 1 / 1;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}
.skill-rank-face--hidden { visibility: hidden; }
.skill-rank-caption { letter-spacing: 0.12em; }
.skill-rank--active .skill-rank-caption { color: var(--success); }
.skill-rank-value { font-size: 26px; font-weight: 800; line-height: 1.05; color: var(--gold); font-variant-numeric: tabular-nums; }
.skill-rank-meta { font-size: 11px; color: var(--muted); white-space: nowrap; font-variant-numeric: tabular-nums; }
.skill-rank-next {
  margin-top: 3px;
  padding-top: 3px;
  border-top: 1px solid var(--frame-soft);
  font-size: 11px;
  font-weight: 800;
  color: var(--text-soft);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.skill-meter { display: flex; flex-direction: column; gap: 6px; }
.skill-meter-head, .skill-meter-foot { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
.skill-meter-value { font-size: 15px; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.skill-meter-foot { font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
.skill-forecast {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin-top: 2px;
  padding-top: 6px;
  border-top: 1px solid var(--frame-soft);
}
.skill-forecast-head { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
.skill-forecast-date { font-size: 16px; font-weight: 800; color: var(--gold); white-space: nowrap; font-variant-numeric: tabular-nums; }
.skill-forecast-sub { font-size: 11px; color: var(--muted); text-align: right; font-variant-numeric: tabular-nums; }
.skill-bar { height: 22px; }
.skill-meter--done .skill-bar { --bar-color: var(--success); }
.skill-meter--done .skill-caption { color: var(--success); }
.skill-tick {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: rgba(20, 18, 30, 0.8);
}
.skill-tick-labels { position: relative; height: 12px; margin-top: -3px; }
.skill-tick-label {
  position: absolute;
  top: 0;
  transform: translateX(-50%);
  font-size: 10px;
  font-weight: 700;
  line-height: 12px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.skill-chart {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px 8px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.3);
}
.skill-chart-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.skill-chart-sub { font-size: 11px; color: var(--muted); }
</style>
