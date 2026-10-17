<template>
  <div class="pb">
    <button v-if="!target" type="button" class="bc-btn pb-pick" @click="$emit('edit')">
      <AppIcon name="edit" :size="14" />{{ t('planner.chooseItem') }}
    </button>

    <template v-else>
      <header class="pb-head">
        <ItemCard
          variant="mini"
          badge
          :name="itemMeta(target).name"
          :icon-asset-name="itemMeta(target).iconAssetName"
          :item-type="itemMeta(target).type"
          :tier="itemMeta(target).tier"
        />
        <div class="pb-head-main">
          <div class="pb-qty" role="group" :aria-label="t('planner.quantity')">
            <button type="button" class="pb-qty-btn" :title="t('planner.less')" :disabled="quantity <= 1" @click="stepQuantity(-1)">
              <AppIcon name="minus" :size="12" />
            </button>
            <input
              class="pb-qty-input"
              type="number"
              min="1"
              inputmode="numeric"
              :value="quantity"
              :aria-label="t('planner.howMany')"
              @change="commitQuantity($event.target)"
              @keydown.enter="$event.target.blur()"
            />
            <button type="button" class="pb-qty-btn" :title="t('planner.more')" @click="stepQuantity(1)">
              <AppIcon name="plus" :size="12" />
            </button>
            <span class="pb-qty-unit">{{ t('common.pcs') }}</span>
          </div>
          <div v-if="rootRecipe" class="pb-head-recipe" :title="rootRecipe.name">
            <template v-if="rootRecipe.skillId">
              <span class="pb-skill">{{ skillLabel(rootRecipe) }}</span>
              <span class="pb-dot">·</span>
            </template>
            <span class="pb-ellipsis">{{ stationLabel(rootRecipe) }}</span>
          </div>
        </div>
      </header>

      <div v-if="loading && !plan" class="pb-state bc-muted-note">{{ t('planner.building') }}</div>
      <div v-else-if="error" class="pb-state bc-error">{{ error }}</div>
      <div v-else-if="plan && !plan.hasRecipe" class="pb-state bc-muted-note">{{ t('planner.noRecipe') }}</div>

      <template v-else-if="plan">
        <div class="pb-bar">
          <div class="bc-seg" role="tablist" :aria-label="t('planner.sections')">
            <button
              v-for="option in viewOptions"
              :key="option.id"
              type="button"
              role="tab"
              class="bc-seg-btn"
              :class="{ active: activeView === option.id }"
              :aria-selected="activeView === option.id ? 'true' : 'false'"
              :title="option.title"
              @click="$emit('update-view', option.id)"
            >{{ option.label }}<span v-if="option.count != null" class="pb-tab-count">{{ option.count }}</span></button>
          </div>
          <div v-if="activeView !== 'tree'" class="pb-progress" :title="t('planner.progress', { done: progress.done, total: progress.total })">
            <span class="pb-progress-text">{{ progress.done }}/{{ progress.total }}</span>
            <span class="pb-progress-track"><span class="pb-progress-fill" :style="{ width: `${progress.pct}%` }"></span></span>
            <button v-if="hasDone" type="button" class="pb-link" :title="t('planner.resetTitle')" @click="$emit('reset-done')">{{ t('planner.reset') }}</button>
          </div>
        </div>

        <div class="pb-scroll" :class="{ 'pb-busy': loading }">
          <div v-if="activeView === 'base'" class="pb-groups" :class="{ 'pb-groups--wide': wide }">
            <section v-for="group in baseGroups" :key="group.id" class="pb-group" :class="{ 'pb-group--collapsed': isCollapsed(group.id) }">
              <button type="button" class="pb-group-head" :aria-expanded="isCollapsed(group.id) ? 'false' : 'true'" @click="toggleCollapsed(group.id)">
                <span class="pb-glyph" aria-hidden="true">{{ groupGlyph(group) }}</span>
                <span class="pb-group-name">{{ groupName(group) }}</span>
                <span class="pb-group-count" :class="{ 'pb-group-count--done': groupDone(group) === group.items.length }">{{ groupDone(group) }}/{{ group.items.length }}</span>
                <AppIcon name="chevron" :size="13" class="pb-chevron" />
              </button>
              <ul v-show="!isCollapsed(group.id)" class="pb-list">
                <li
                  v-for="mat in group.items"
                  :key="mat.key"
                  class="pb-row"
                  :class="{ 'pb-row--done': isDone(mat.key) }"
                  @click="$emit('toggle-done', mat.key)"
                >
                  <span class="pb-check" role="checkbox" :aria-checked="isDone(mat.key) ? 'true' : 'false'" :title="isDone(mat.key) ? t('planner.uncheck') : t('planner.checkGathered')">
                    <AppIcon v-if="isDone(mat.key)" name="check" :size="11" />
                  </span>
                  <ItemCard
                    variant="mini"
                    :name="itemMeta(mat).name"
                    :icon-asset-name="itemMeta(mat).iconAssetName"
                    :item-type="itemMeta(mat).type"
                    :tier="itemMeta(mat).tier"
                  />
                  <span class="pb-name">{{ itemMeta(mat).name }}</span>
                  <span v-if="mat.source?.level > 1" class="pb-level" :title="t('planner.requires', { skill: skillName(mat.source.skillId), level: mat.source.level })">{{ t('planner.lvl', { level: mat.source.level }) }}</span>
                  <button
                    v-if="mat.override === BASE"
                    type="button"
                    class="pb-chip"
                    :title="t('planner.restoreCraftTitle')"
                    @click.stop="$emit('override', { key: mat.key, value: null })"
                  >{{ t('planner.craft') }}</button>
                  <span class="pb-amount">{{ formatQty(mat.quantity) }}</span>
                </li>
              </ul>
            </section>
          </div>

          <div v-else-if="activeView === 'steps'" class="pb-groups" :class="{ 'pb-groups--wide': wide }">
            <section v-for="group in craftGroups" :key="group.id" class="pb-group" :class="{ 'pb-group--collapsed': isCollapsed(group.id) }">
              <button type="button" class="pb-group-head" :aria-expanded="isCollapsed(group.id) ? 'false' : 'true'" @click="toggleCollapsed(group.id)">
                <span class="pb-glyph" aria-hidden="true">{{ groupGlyph(group) }}</span>
                <span class="pb-group-name">{{ groupName(group) }}</span>
                <span v-if="group.maxLevel > 1" class="pb-group-meta" :title="t('planner.maxLevelTitle')">{{ t('planner.upToLvl', { level: group.maxLevel }) }}</span>
                <span class="pb-group-count" :class="{ 'pb-group-count--done': groupDone(group) === group.items.length }">{{ groupDone(group) }}/{{ group.items.length }}</span>
                <AppIcon name="chevron" :size="13" class="pb-chevron" />
              </button>
              <ul v-show="!isCollapsed(group.id)" class="pb-list">
                <li
                  v-for="step in group.items"
                  :key="step.key"
                  class="pb-row pb-row--step"
                  :class="{ 'pb-row--done': isDone(step.key), 'pb-row--root': step.isRoot }"
                  @click="$emit('toggle-done', step.key)"
                >
                  <span class="pb-check" role="checkbox" :aria-checked="isDone(step.key) ? 'true' : 'false'" :title="isDone(step.key) ? t('planner.uncheck') : t('planner.checkCrafted')">
                    <AppIcon v-if="isDone(step.key)" name="check" :size="11" />
                  </span>
                  <ItemCard
                    variant="mini"
                    :name="itemMeta(step).name"
                    :icon-asset-name="itemMeta(step).iconAssetName"
                    :item-type="itemMeta(step).type"
                    :tier="itemMeta(step).tier"
                  />
                  <div class="pb-step">
                    <div class="pb-step-line">
                      <span class="pb-name">{{ itemMeta(step).name }}</span>
                      <span v-if="step.surplus > 0" class="pb-surplus" :title="t('planner.surplus', { produced: formatQty(step.produced), surplus: formatQty(step.surplus) })">+{{ formatQty(step.surplus) }}</span>
                      <span class="pb-amount">{{ formatQty(step.needed) }}</span>
                    </div>
                    <div class="pb-step-line pb-step-meta">
                      <span class="pb-crafts" :title="t('planner.crafts', { n: step.crafts, out: step.recipe.outputQuantity })">{{ formatQty(step.crafts) }}×</span>
                      <label
                        v-if="step.options.length > 1 || !step.isRoot"
                        class="pb-recipe"
                        :title="t('planner.changeRecipe', { name: step.recipe.name })"
                        @click.stop
                      >
                        <span class="pb-ellipsis">{{ stationLabel(step.recipe) }}</span>
                        <AppIcon name="chevron" :size="11" class="pb-recipe-chevron" />
                        <select
                          class="pb-recipe-select"
                          :value="String(step.recipe.id)"
                          :aria-label="t('common.recipe')"
                          @change="$emit('override', { key: step.key, value: $event.target.value, isRoot: step.isRoot })"
                        >
                          <option v-for="opt in step.options" :key="opt.id" :value="String(opt.id)">{{ optionLabel(opt, step.options) }}</option>
                          <option v-if="!step.isRoot" :value="BASE">{{ t('planner.dontCraft') }}</option>
                        </select>
                      </label>
                      <span v-else class="pb-ellipsis">{{ stationLabel(step.recipe) }}</span>
                      <span v-if="step.recipe.skillLevel > 1 && step.recipe.skillLevel !== group.maxLevel" class="pb-tag" :title="t('planner.requires', { skill: skillName(step.recipe.skillId), level: step.recipe.skillLevel })">{{ t('planner.lvl', { level: step.recipe.skillLevel }) }}</span>
                      <span v-if="step.recipe.passive" class="pb-tag pb-tag--passive" :title="t('planner.passiveTitle')">⏱ {{ formatDuration(step.recipe.timeRequirement) }}</span>
                      <button
                        v-if="!step.isRoot && step.override"
                        type="button"
                        class="pb-reset"
                        :title="t('planner.defaultRecipe')"
                        @click.stop="$emit('override', { key: step.key, value: null })"
                      ><AppIcon name="refresh" :size="11" /></button>
                    </div>
                  </div>
                </li>
              </ul>
            </section>
          </div>

          <ul v-else class="pb-list pb-tree">
            <li
              v-for="(row, idx) in treeRows"
              :key="`${row.itemType}-${row.id}-${row.depth}-${idx}`"
              class="pb-row pb-tree-row"
              :class="{ 'pb-tree-row--nested': row.depth > 0, 'pb-tree-row--leaf': !row.children.length }"
              :style="{ paddingLeft: `${4 + row.depth * 16}px`, '--indent': `${row.depth * 16 - 8}px` }"
            >
              <ItemCard
                variant="mini"
                :name="itemMeta(row).name"
                :icon-asset-name="itemMeta(row).iconAssetName"
                :item-type="itemMeta(row).type"
                :tier="itemMeta(row).tier"
              />
              <span class="pb-name">{{ itemMeta(row).name }}</span>
              <span v-if="row.crafts" class="pb-tree-crafts">{{ formatQty(row.crafts) }}×</span>
              <span class="pb-amount">{{ formatQty(row.quantity) }}</span>
            </li>
            <li v-if="treeTruncated" class="bc-muted-note pb-state">{{ t('planner.treeTooBig') }}</li>
          </ul>
        </div>
      </template>
    </template>
  </div>
</template>

<script>
import { computed, reactive } from 'vue'
import {
  PLANNER_BASE_OVERRIDE,
  SKILL_NAMES,
  flattenCraftTree,
  groupBaseResources,
  groupCraftSteps,
} from '../plannerCrafting'
import { skillGlyph } from '../skillProgress'
import { formatNumber, t } from '../i18n'
import AppIcon from './AppIcon.vue'
import ItemCard from './ItemCard.vue'

const VIEWS = ['base', 'steps', 'tree']

function hasTruncated(node) {
  return Boolean(node?.truncated || node?.children?.some(hasTruncated))
}

export default {
  name: 'PlannerBlock',
  components: { AppIcon, ItemCard },
  props: {
    target: { type: Object, default: null },
    plan: { type: Object, default: null },
    quantity: { type: Number, default: 1 },
    loading: { type: Boolean, default: false },
    error: { type: String, default: null },
    view: { type: String, default: 'base' },
    done: { type: Object, default: () => ({}) },
    itemMeta: { type: Function, required: true },
    wide: { type: Boolean, default: false },
  },
  emits: ['edit', 'update-view', 'update-quantity', 'override', 'toggle-done', 'reset-done'],
  setup(props, { emit }) {
    const collapsed = reactive({})

    const activeView = computed(() => (VIEWS.includes(props.view) ? props.view : 'base'))
    const rootRecipe = computed(() => props.plan?.root?.recipe ?? null)
    const baseGroups = computed(() => groupBaseResources(props.plan?.base))
    const craftGroups = computed(() =>
      groupCraftSteps(props.plan?.steps, (step) => props.itemMeta(step).tier),
    )
    const treeRows = computed(() => flattenCraftTree(props.plan?.tree?.children ?? []))
    const treeTruncated = computed(() => hasTruncated(props.plan?.tree))
    const viewOptions = computed(() => [
      { id: 'base', label: t('planner.baseTab'), title: t('planner.baseTabTitle'), count: props.plan?.base.length ?? 0 },
      { id: 'steps', label: t('planner.stepsTab'), title: t('planner.stepsTabTitle'), count: props.plan?.steps.length ?? 0 },
      { id: 'tree', label: t('planner.treeTab'), title: t('planner.treeTabTitle'), count: null },
    ])
    const hasDone = computed(() => Object.keys(props.done ?? {}).length > 0)
    const progress = computed(() => {
      const rows = activeView.value === 'steps' ? props.plan?.steps ?? [] : props.plan?.base ?? []
      const done = rows.filter((row) => isDone(row.key)).length
      const total = rows.length
      return { done, total, pct: total ? Math.round((done / total) * 100) : 0 }
    })

    function isDone(key) {
      return Boolean(props.done?.[key])
    }
    function groupDone(group) {
      return group.items.filter((row) => isDone(row.key)).length
    }
    function isCollapsed(groupId) {
      return Boolean(collapsed[`${activeView.value}:${groupId}`])
    }
    function toggleCollapsed(groupId) {
      const key = `${activeView.value}:${groupId}`
      collapsed[key] = !collapsed[key]
    }
    function skillName(skillId) {
      return SKILL_NAMES[skillId] ?? t('planner.skill', { id: skillId })
    }
    function skillLabel(recipe) {
      const name = `${skillGlyph(recipe.skillId)} ${skillName(recipe.skillId)}`
      return recipe.skillLevel > 1 ? `${name} ${recipe.skillLevel}` : name
    }
    function groupName(group) {
      if (group.kind === 'skill') return skillName(group.skillId)
      if (group.kind === 'trader') return t('planner.traders')
      return t('planner.other')
    }
    function groupGlyph(group) {
      if (group.kind === 'skill') return skillGlyph(group.skillId)
      if (group.kind === 'trader') return '🪙'
      return '📦'
    }
    function stationLabel(recipe) {
      if (!recipe?.buildingName) return recipe?.name ?? t('common.recipe')
      return recipe.buildingTier ? `${recipe.buildingName} T${recipe.buildingTier}` : recipe.buildingName
    }
    function optionLabel(option, options) {
      const station = stationLabel(option)
      const twins = options.filter((o) => stationLabel(o) === station)
      if (twins.length < 2) return station
      return `${station} · ×${option.outputQuantity} · ${t('planner.ingredients', { n: option.ingredientCount })}`
    }
    function formatQty(value) {
      const n = Number(value)
      if (!Number.isFinite(n)) return '—'
      return formatNumber(n)
    }
    function formatDuration(seconds) {
      const s = Math.round(Number(seconds) || 0)
      if (s < 60) return t('units.seconds', { n: s })
      if (s < 3600) return t('units.minutes', { n: Math.round(s / 60) })
      const h = Math.floor(s / 3600)
      const m = Math.round((s % 3600) / 60)
      return m ? t('units.hoursMinutes', { h, m }) : t('units.hours', { n: h })
    }
    function stepQuantity(delta) {
      emit('update-quantity', Math.max(1, props.quantity + delta))
    }
    function commitQuantity(input) {
      const value = Math.max(1, Math.floor(Number(input.value) || 1))
      input.value = value
      if (value !== props.quantity) emit('update-quantity', value)
    }

    return {
      BASE: PLANNER_BASE_OVERRIDE,
      activeView, rootRecipe, baseGroups, craftGroups, treeRows, treeTruncated, viewOptions,
      hasDone, progress,
      isDone, groupDone, isCollapsed, toggleCollapsed,
      skillName, skillLabel, groupName, groupGlyph, stationLabel, optionLabel,
      formatQty, formatDuration, stepQuantity, commitQuantity,
    }
  },
}
</script>

<style scoped>
.pb { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; }
.pb-pick { align-self: center; margin-top: 24px; }
.pb-state { padding: 4px 2px; }

.pb-head { display: flex; align-items: center; gap: 12px; }
.pb-head :deep(.item-card--mini .item-card__icon) { width: 44px; height: 52px; flex-basis: 44px; }
.pb-head-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 5px; }
.pb-qty { display: inline-flex; align-items: center; gap: 4px; }
.pb-qty-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 26px;
  padding: 0;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.35);
  color: var(--text-soft);
  cursor: pointer;
}
.pb-qty-btn:hover:not(:disabled) { border-color: var(--cream); color: var(--text); }
.pb-qty-btn:disabled { opacity: 0.35; cursor: default; }
.pb-qty-input {
  width: 64px;
  height: 26px;
  padding: 0 6px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.45);
  color: var(--text);
  font-size: 16px;
  font-weight: 800;
  text-align: center;
  font-variant-numeric: tabular-nums;
  outline: none;
  appearance: textfield;
  -moz-appearance: textfield;
}
.pb-qty-input::-webkit-outer-spin-button,
.pb-qty-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.pb-qty-input:focus { border-color: var(--cream); }
.pb-qty-unit { margin-left: 2px; font-size: 12px; color: var(--muted); }
.pb-head-recipe { display: flex; align-items: center; gap: 6px; min-width: 0; font-size: 12px; color: var(--muted); }
.pb-skill { flex-shrink: 0; color: var(--text-soft); font-weight: 600; }
.pb-dot { flex-shrink: 0; opacity: 0.6; }
.pb-ellipsis { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.pb-bar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.pb-tab-count { margin-left: 5px; opacity: 0.65; font-variant-numeric: tabular-nums; }
.pb-progress { flex: 1; min-width: 110px; display: flex; align-items: center; gap: 8px; }
.pb-progress-text { flex-shrink: 0; font-size: 11px; font-weight: 700; color: var(--text-soft); font-variant-numeric: tabular-nums; }
.pb-progress-track { flex: 1; height: 4px; border-radius: 2px; background: rgba(233, 223, 196, 0.1); overflow: hidden; }
.pb-progress-fill { display: block; height: 100%; background: var(--success); transition: width 0.25s; }
.pb-link {
  flex-shrink: 0;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  font-size: 11px;
  text-decoration: underline dotted;
  text-underline-offset: 3px;
  cursor: pointer;
}
.pb-link:hover { color: var(--text); }

.pb-scroll { flex: 1; min-height: 0; margin-right: -6px; padding-right: 6px; overflow: auto; }
.pb-busy { opacity: 0.55; pointer-events: none; transition: opacity 0.15s; }

.pb-groups { display: flex; flex-direction: column; gap: 10px; }
.pb-groups--wide { display: block; columns: 2; column-gap: 20px; }
.pb-groups--wide .pb-group { break-inside: avoid; margin-bottom: 10px; }
.pb-group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 5px 4px 5px 2px;
  border: none;
  border-bottom: 1px solid var(--frame-soft);
  background: none;
  color: var(--text-soft);
  text-align: left;
  cursor: pointer;
}
.pb-group-head:hover { color: var(--text); }
.pb-glyph { flex-shrink: 0; width: 20px; font-size: 14px; line-height: 1; text-align: center; }
.pb-group-name {
  flex: 1;
  min-width: 0;
  font-family: var(--font-title);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pb-group-meta { flex-shrink: 0; font-size: 11px; color: var(--muted); }
.pb-group-count { flex-shrink: 0; font-size: 11px; font-weight: 700; color: var(--muted); font-variant-numeric: tabular-nums; }
.pb-group-count--done { color: var(--success); }
.pb-chevron { flex-shrink: 0; color: var(--muted); transition: transform 0.15s; }
.pb-group--collapsed .pb-chevron { transform: rotate(-90deg); }

.pb-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.pb-row {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 9px;
  min-height: 36px;
  padding: 3px 4px;
  border-bottom: 1px solid rgba(233, 223, 196, 0.06);
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.12s;
}
.pb-row:hover { background: rgba(233, 223, 196, 0.05); }
.pb-row :deep(.item-card--mini .item-card__icon) { width: 26px; height: 30px; flex-basis: 26px; border-width: 1px; border-radius: 4px; }
.pb-check {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1px solid var(--border-strong);
  border-radius: 3px;
  color: var(--on-cream);
  transition: background 0.12s, border-color 0.12s;
}
.pb-row:hover .pb-check { border-color: var(--cream); }
.pb-row--done .pb-check { border-color: var(--success); background: var(--success); }
.pb-name { flex: 1; min-width: 0; font-size: 12px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pb-amount { flex-shrink: 0; font-size: 13px; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.pb-level { flex-shrink: 0; font-size: 10px; color: var(--muted); }
.pb-row--done .pb-name,
.pb-row--done .pb-amount { color: var(--muted); text-decoration: line-through; text-decoration-color: rgba(163, 157, 152, 0.6); }
.pb-row--done :deep(.item-card) { opacity: 0.45; }

.pb-row--step { align-items: center; padding-top: 5px; padding-bottom: 5px; }
.pb-row--root .pb-name { color: var(--cream); font-weight: 700; }
.pb-step { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.pb-step-line { display: flex; align-items: center; gap: 6px; min-width: 0; }
.pb-step-meta { font-size: 11px; color: var(--muted); }
.pb-crafts { flex-shrink: 0; font-weight: 700; color: var(--text-soft); font-variant-numeric: tabular-nums; }
.pb-surplus { flex-shrink: 0; font-size: 11px; font-weight: 700; color: var(--gold); font-variant-numeric: tabular-nums; }
.pb-recipe {
  position: relative;
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 20px;
  padding: 0 4px;
  margin-left: -4px;
  border: 1px solid transparent;
  border-radius: 3px;
  color: var(--muted);
  cursor: pointer;
}
.pb-recipe:hover, .pb-recipe:focus-within { border-color: var(--border-strong); color: var(--text); }
.pb-recipe-chevron { flex-shrink: 0; opacity: 0.7; }
.pb-recipe-select {
  position: absolute;
  inset: 0;
  width: 100%;
  opacity: 0;
  cursor: pointer;
  font-size: 12px;
}
.pb-recipe-select option { background: var(--panel); color: var(--text); }
.pb-tag {
  flex-shrink: 0;
  padding: 0 5px;
  border: 1px solid rgba(233, 223, 196, 0.14);
  border-radius: 3px;
  font-size: 10px;
  line-height: 15px;
  color: var(--muted-strong);
  white-space: nowrap;
}
.pb-tag--passive { border-color: rgba(94, 166, 220, 0.35); color: var(--accent); }
.pb-reset,
.pb-chip {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--border-strong);
  border-radius: 3px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.pb-reset { padding: 2px; }
.pb-chip { padding: 1px 6px; font-size: 10px; }
.pb-reset:hover, .pb-chip:hover { border-color: var(--cream); color: var(--text); }

.pb-tree-row { position: relative; cursor: default; }
.pb-tree-row--nested::before {
  content: '';
  position: absolute;
  left: var(--indent, 0px);
  top: 0;
  bottom: 50%;
  width: 8px;
  border-left: 1px solid var(--border-strong);
  border-bottom: 1px solid var(--border-strong);
}
.pb-tree-row--leaf .pb-name,
.pb-tree-row--leaf .pb-amount { color: var(--text-soft); }
.pb-tree-crafts { flex-shrink: 0; font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; }
</style>
