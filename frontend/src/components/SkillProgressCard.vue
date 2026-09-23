<template>
  <div v-if="!progress" class="empty-inv">нет данных</div>
  <div v-else class="skill-card" :class="`skill-card--${size}`">
    <div class="skill-summary">
      <div class="skill-hero">
        <div class="skill-hero-level">
          <span class="skill-caption"><span class="skill-glyph">{{ glyph }}</span> {{ skillTitle || 'Уровень' }}</span>
          <span class="skill-level">
            {{ progress.level }}<span class="skill-level-cap">/ {{ progress.targetLevel }}</span>
          </span>
        </div>
        <div class="skill-rank" :title="rankTitle">
          <span class="skill-caption">Место</span>
          <span class="skill-rank-value">{{ rank ? `#${formatCount(rank)}` : '—' }}</span>
          <span class="skill-rank-meta">
            <template v-if="rank && totalPlayers">из {{ formatCount(totalPlayers) }}</template>
            <template v-else>нет в рейтинге</template>
          </span>
          <span v-if="topPct != null" class="skill-rank-top">топ {{ formatTopPct(topPct) }}</span>
        </div>
      </div>

      <div class="skill-meter">
        <div class="skill-meter-head">
          <span class="skill-meter-label">{{ progress.isMax ? 'Максимальный уровень' : `До ${progress.level + 1} уровня` }}</span>
          <span class="skill-meter-value">{{ formatSkillPct(progress.levelPct) }}</span>
        </div>
        <div class="skill-track">
          <div class="skill-fill" :style="{ width: `${progress.levelPct}%` }"></div>
        </div>
        <div v-if="!progress.isMax" class="skill-meter-foot">
          <span>{{ formatXpCompact(progress.levelXp) }} / {{ formatXpCompact(progress.levelSpan) }} XP</span>
          <span :title="etaTitle">ещё {{ formatXpCompact(progress.toNext) }}<template v-if="etaNext"> · ≈ {{ etaNext }}</template></span>
        </div>
      </div>

      <div class="skill-meter skill-meter--path" :class="{ 'skill-meter--done': progress.reachedTarget }">
        <div class="skill-meter-head">
          <span class="skill-meter-label">
            <template v-if="progress.reachedTarget">✓ Цель {{ progress.targetLevel }} достигнута</template>
            <template v-else>Путь до {{ progress.targetLevel }} по опыту</template>
          </span>
          <span class="skill-meter-value">{{ formatSkillPct(progress.targetPct) }}</span>
        </div>
        <div class="skill-track skill-track--path">
          <div class="skill-fill" :style="{ width: `${progress.targetPct}%` }"></div>
          <span
            v-for="tick in pathTicks"
            :key="tick.level"
            class="skill-track-tick"
            :style="{ left: `${tick.pct}%` }"
          ></span>
        </div>
        <div class="skill-tick-labels">
          <span
            v-for="tick in pathTicks"
            :key="tick.level"
            class="skill-tick-label"
            :style="{ left: `${tick.pct}%` }"
          >{{ tick.level }}</span>
          <span class="skill-tick-label skill-tick-label--end">{{ progress.targetLevel }}</span>
        </div>
        <div class="skill-meter-foot">
          <span>{{ formatXpCompact(progress.xp) }} / {{ formatXpCompact(progress.targetXp) }} XP</span>
          <span v-if="!progress.reachedTarget" :title="etaTitle">ещё {{ formatXpCompact(progress.toTarget) }}<template v-if="etaTarget"> · ≈ {{ etaTarget }}</template></span>
          <span v-else>+{{ formatXpCompact(progress.xp - progress.targetXp) }} сверх цели</span>
        </div>
      </div>

    </div>

    <div v-if="size === 'large'" class="skill-chart">
      <div class="skill-chart-head">
        <div class="skill-tabs">
          <button type="button" class="skill-tab" :class="{ active: activeTab === 'pace' }" @click="$emit('update-view', { skillChartTab: 'pace' })">Темп</button>
          <button type="button" class="skill-tab" :class="{ active: activeTab === 'curve' }" @click="$emit('update-view', { skillChartTab: 'curve' })">Кривая</button>
        </div>
        <span v-if="activeTab === 'curve'" class="skill-chart-sub">всего XP для уровня</span>
      </div>
      <SkillPacePanel
        v-if="activeTab === 'pace'"
        :points="history"
        :level-table="levelTable"
        :level="progress.level"
        :now="nowSec"
        :period="pacePeriod"
        :mode="paceMode"
        :tracked-since="trackedSince"
        @update:mode="$emit('update-view', { skillPaceMode: $event })"
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
  skillGlyph,
  skillProgress,
  targetPathTicks,
} from '../skillProgress'
import { activePace, formatPlayTime } from '../xpHistory'
import SkillCurveChart from './SkillCurveChart.vue'
import SkillPacePanel from './SkillPacePanel.vue'

const NOW_TICK_MS = 15000

export default {
  name: 'SkillProgressCard',
  components: { SkillCurveChart, SkillPacePanel },
  props: {
    levelTable: { type: Array, default: null },
    xp: { type: Number, default: 0 },
    rank: { type: Number, default: null },
    totalPlayers: { type: Number, default: null },
    skillId: { type: [String, Number], default: null },
    skillTitle: { type: String, default: '' },
    size: { type: String, default: 'small' },
    history: { type: Array, default: null },
    trackedSince: { type: Number, default: null },
    chartTab: { type: String, default: 'pace' },
    pacePeriod: { type: String, default: '24h' },
    paceMode: { type: String, default: 'gain' },
  },
  emits: ['update-view'],
  setup(props) {
    const nowSec = ref(Math.floor(Date.now() / 1000))
    let nowTimer = null
    onMounted(() => { nowTimer = setInterval(() => { nowSec.value = Math.floor(Date.now() / 1000) }, NOW_TICK_MS) })
    onUnmounted(() => clearInterval(nowTimer))

    const progress = computed(() => skillProgress(props.levelTable, props.xp))
    const glyph = computed(() => skillGlyph(props.skillId))
    const pathTicks = computed(() => (progress.value ? targetPathTicks(props.levelTable, progress.value.targetLevel) : []))
    const topPct = computed(() => (props.rank && props.totalPlayers ? (props.rank / props.totalPlayers) * 100 : null))
    const activeTab = computed(() => (props.chartTab === 'curve' ? 'curve' : 'pace'))
    // "Hours of play" pace: last 7 days, or everything recorded if that is all there is.
    const playPace = computed(() => activePace(props.history, nowSec.value, 7 * 86400) ?? activePace(props.history, nowSec.value, null))
    const etaFor = (remaining) => (playPace.value?.perHour > 0 ? formatPlayTime((remaining / playPace.value.perHour) * 3600) : '')
    const etaNext = computed(() => (progress.value && !progress.value.isMax ? etaFor(progress.value.toNext) : ''))
    const etaTarget = computed(() => (progress.value && !progress.value.reachedTarget ? etaFor(progress.value.toTarget) : ''))
    const etaTitle = computed(() => (playPace.value
      ? `Время активной игры в вашем темпе: ${formatXpCompact(playPace.value.perHour)} XP/ч`
      : ''))
    const rankTitle = computed(() => {
      if (!props.rank) return 'Нет опыта в этом навыке — игрок не попал в рейтинг'
      return `Место #${props.rank} среди ${props.totalPlayers ?? '?'} игроков с опытом в навыке`
    })

    function formatCount(n) {
      return Number(n ?? 0).toLocaleString('ru-RU')
    }

    return {
      nowSec, progress, glyph, pathTicks, topPct, activeTab, etaNext, etaTarget, etaTitle, rankTitle,
      formatCount, formatSkillPct, formatTopPct, formatXpCompact,
    }
  },
}
</script>

<style scoped>
.empty-inv { font-size: 13px; color: var(--muted); font-style: italic; }
.skill-card {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 22px;
}
.skill-card--small { flex-direction: column; justify-content: center; }
.skill-summary {
  display: flex;
  flex-direction: column;
  gap: 16px;
  justify-content: center;
  min-width: 0;
}
.skill-card--small .skill-summary { flex: 1; }
.skill-card--large .skill-summary { flex: 0 0 300px; gap: 14px; }

.skill-hero { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.skill-hero-level { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.skill-caption {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
  white-space: nowrap;
}
.skill-glyph { font-size: 13px; letter-spacing: 0; }
.skill-level { font-size: 52px; font-weight: 700; line-height: 1; color: var(--text); }
.skill-level-cap { margin-left: 6px; font-size: 18px; font-weight: 600; color: var(--muted); }

.skill-rank {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 3px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-raised);
  text-align: right;
}
.skill-rank-value { font-size: 24px; font-weight: 700; line-height: 1.05; color: var(--text); }
.skill-rank-meta { font-size: 11px; color: var(--muted); }
.skill-rank-top {
  margin-top: 2px;
  padding: 1px 7px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent);
  background: rgba(138, 164, 255, 0.16);
}

.skill-meter { display: flex; flex-direction: column; gap: 6px; }
.skill-meter-head, .skill-meter-foot { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
.skill-meter-label { font-size: 13px; font-weight: 600; color: var(--text-soft); }
.skill-meter-value { font-size: 15px; font-weight: 700; color: var(--text); font-variant-numeric: tabular-nums; }
.skill-meter-foot { font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; }
.skill-track {
  position: relative;
  height: 8px;
  border-radius: 999px;
  background: rgba(138, 164, 255, 0.12);
  border: 1px solid var(--border);
  overflow: hidden;
}
.skill-track--path { height: 12px; }
.skill-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent-strong), var(--accent));
  box-shadow: 0 0 8px rgba(138, 164, 255, 0.4);
  transition: width 0.5s ease;
}
.skill-meter--done .skill-fill {
  background: linear-gradient(90deg, #4bc98a, var(--success));
  box-shadow: 0 0 8px rgba(117, 224, 167, 0.4);
}
.skill-meter--done .skill-meter-label { color: var(--success); }
.skill-track-tick {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: var(--surface);
}
.skill-tick-labels { position: relative; height: 12px; margin-top: -2px; }
.skill-tick-label {
  position: absolute;
  top: 0;
  transform: translateX(-50%);
  font-size: 10px;
  font-weight: 600;
  line-height: 12px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
.skill-tick-label--end { left: auto; right: 0; transform: none; }



.skill-chart {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px 8px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: rgba(37, 36, 55, 0.45);
}
.skill-chart-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.skill-chart-sub { font-size: 11px; color: var(--muted); }
.skill-tabs { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.skill-tab {
  border: none;
  background: var(--surface-raised);
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
  padding: 3px 9px;
  cursor: pointer;
}
.skill-tab.active { background: rgba(138, 164, 255, 0.2); color: var(--accent); }
</style>
