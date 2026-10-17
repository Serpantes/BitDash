<template>
  <div class="msearch">
    <div class="msearch-row">
      <label class="msearch-field" :class="{ 'msearch-field--open': open }">
        <AppIcon name="search" :size="14" class="msearch-icon" />
        <input
          ref="inputRef"
          :value="query"
          class="msearch-input"
          type="text"
          role="combobox"
          aria-autocomplete="list"
          aria-haspopup="listbox"
          :aria-expanded="open ? 'true' : 'false'"
          :aria-controls="listId"
          :aria-activedescendant="open && activeEntry ? optionId(activeEntry.index) : undefined"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          enterkeyhint="search"
          :placeholder="t('marketSearch.placeholder')"
          @input="onInput"
          @focus="onFocus"
          @blur="open = false"
          @keydown="onKeydown"
        />
        <span v-if="marketGoodsLoading && !marketGoods" class="msearch-spinner" role="status" :aria-label="t('marketSearch.loadingCatalogAria')"></span>
        <button
          v-else-if="query"
          type="button"
          class="msearch-clear"
          :title="t('common.clearEsc')"
          :aria-label="t('common.clear')"
          @mousedown.prevent
          @click="clear"
        ><AppIcon name="close" :size="12" /></button>
      </label>
      <slot name="aside" />
    </div>

    <div v-if="open" :id="listId" ref="popRef" class="msearch-pop" role="listbox" :aria-label="t('marketSearch.listAria')" @mousedown.prevent>
      <template v-if="view.sections">
        <template v-for="section in view.sections" :key="section.id">
          <div class="msearch-section" role="presentation">
            <span>{{ section.title }}</span>
            <span v-if="section.note" class="msearch-section-note">{{ section.note }}</span>
            <button v-if="section.id === 'recent'" type="button" class="msearch-section-action" @click="$emit('clear-recent')">{{ t('marketSearch.clearRecent') }}</button>
          </div>
          <div
            v-for="entry in section.entries"
            :id="optionId(entry.index)"
            :key="entry.good.key"
            role="option"
            :aria-selected="entry.index === activeIndex ? 'true' : 'false'"
            class="msearch-opt"
            :class="{ 'is-active': entry.index === activeIndex, 'is-quiet': !entry.traded }"
            @mousemove="activeIndex = entry.index"
            @click="choose(entry.good)"
          >
            <span class="msearch-thumb">
              <ItemCard
                variant="mini"
                :name="entry.good.name"
                :icon-asset-name="entry.good.iconAssetName"
                :item-type="entry.good.itemType"
                :tier="entry.good.tier"
                :rarity-str="entry.good.rarityStr"
              />
            </span>
            <span class="msearch-opt-text">
              <span class="msearch-opt-name"><template v-for="(part, i) in entry.parts" :key="i"><mark v-if="part.hit">{{ part.text }}</mark><template v-else>{{ part.text }}</template></template></span>
              <span class="msearch-opt-meta">{{ entry.meta }}</span>
            </span>
            <span v-if="entry.traded" class="msearch-opt-flags">
              <span v-if="entry.good.hasSell" class="msearch-flag msearch-flag--sell">{{ t('marketSearch.sells') }}</span>
              <span v-if="entry.good.hasBuy" class="msearch-flag msearch-flag--buy">{{ t('marketSearch.buys') }}</span>
            </span>
          </div>
        </template>
        <div v-if="view.more" class="msearch-more">
          {{ t('marketSearch.more', { shown: view.shown, total: view.total }) }}
        </div>
      </template>

      <div v-else class="msearch-message" role="presentation">
        <template v-if="view.kind === 'loading'">{{ t('marketSearch.loadingCatalog') }}</template>
        <template v-else-if="view.kind === 'error'">
          <span class="bc-error">{{ t('marketSearch.catalogFailed', { error: marketGoodsError }) }}</span>
          <button type="button" class="bc-btn msearch-retry" @click="retry">{{ t('common.retry') }}</button>
        </template>
        <template v-else-if="view.kind === 'empty'">
          <span>{{ t('marketSearch.notFound', { query: query.trim() }) }}</span>
          <span class="msearch-hint">{{ view.hint }}</span>
        </template>
        <template v-else>
          <span>{{ t('marketSearch.startTyping') }}</span>
          <span class="msearch-hint">{{ t('marketSearch.hintNames') }} <b>plank</b>, <b>ingot</b>. {{ t('marketSearch.hintTier') }} <b>plank t3</b>.</span>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ensureMarketGoods, marketGoodByKey, marketGoods, marketGoodsError, marketGoodsLoading } from '../marketCatalog'
import { goodMeta, highlightParts, queryHasCyrillic, searchGoods } from '../marketSearch'
import AppIcon from './AppIcon.vue'
import ItemCard from './ItemCard.vue'
import { t } from '../i18n'

let uid = 0

export default {
  name: 'MarketSearch',
  components: { AppIcon, ItemCard },
  props: {
    recent: { type: Array, default: () => [] },
  },
  emits: ['select', 'clear-recent'],
  expose: ['focus'],
  setup(props, { emit }) {
    const listId = `msearch-${++uid}`
    const inputRef = ref(null)
    const popRef = ref(null)
    const query = ref('')
    const open = ref(false)
    const activeIndex = ref(0)

    const results = computed(() => searchGoods(marketGoods.value, query.value))

    const view = computed(() => {
      if (!marketGoods.value) return { kind: marketGoodsError.value && !marketGoodsLoading.value ? 'error' : 'loading' }
      const q = query.value.trim()
      if (!q) {
        const recent = props.recent.map((good) => marketGoodByKey(goodKey(good)) ?? { ...good, key: goodKey(good) })
        if (!recent.length) return { kind: 'hint' }
        return withIndexes([{ id: 'recent', title: t('marketSearch.recent'), goods: recent }], q)
      }
      const found = results.value
      if (!found || !found.total) {
        return {
          kind: 'empty',
          hint: queryHasCyrillic(q)
            ? t('marketSearch.cyrillicHint')
            : t('marketSearch.spellingHint'),
        }
      }
      const sections = []
      if (found.traded.length) sections.push({ id: 'traded', title: t('marketSearch.traded'), goods: found.traded })
      if (found.untraded.length) sections.push({ id: 'untraded', title: t('marketSearch.untraded'), note: t('marketSearch.untradedNote'), goods: found.untraded })
      const shown = found.traded.length + found.untraded.length
      return { ...withIndexes(sections, q), more: found.total > shown, shown, total: found.total }
    })

    function withIndexes(sections, q) {
      let index = 0
      return {
        kind: 'list',
        sections: sections.map((section) => ({
          ...section,
          entries: section.goods.map((good) => ({
            good,
            index: index++,
            traded: Boolean(good.hasSell || good.hasBuy),
            parts: highlightParts(good.name, q),
            meta: goodMeta(good),
          })),
        })),
        count: index,
      }
    }
    const entries = computed(() => (view.value.sections ?? []).flatMap((section) => section.entries))
    const activeEntry = computed(() => entries.value[activeIndex.value] ?? null)

    watch(query, () => { activeIndex.value = 0 })

    function goodKey(good) {
      return good?.key ?? `${Number(good?.itemType) === 1 ? 1 : 0}:${good?.id}`
    }
    function optionId(index) {
      return `${listId}-opt-${index}`
    }
    function onInput(event) {
      query.value = event.target.value
      open.value = true
    }
    function onFocus() {
      ensureMarketGoods()
      open.value = true
    }
    function choose(good) {
      if (!good) return
      open.value = false
      emit('select', good)
    }
    function clear() {
      query.value = ''
      open.value = true
      inputRef.value?.focus()
    }
    function retry() {
      ensureMarketGoods()
    }
    function move(step) {
      const count = entries.value.length
      if (!count) return
      activeIndex.value = (activeIndex.value + step + count) % count
      nextTick(() => {
        popRef.value?.querySelector(`#${optionId(activeIndex.value)}`)?.scrollIntoView({ block: 'nearest' })
      })
    }
    function onKeydown(event) {
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault()
        if (!open.value) open.value = true
        else move(event.key === 'ArrowDown' ? 1 : -1)
      } else if (event.key === 'Enter') {
        if (open.value && activeEntry.value) {
          event.preventDefault()
          choose(activeEntry.value.good)
        }
      } else if (event.key === 'Escape') {
        if (open.value) open.value = false
        else if (query.value) query.value = ''
        else inputRef.value?.blur()
      }
    }
    function focus() {
      inputRef.value?.focus()
    }

    onMounted(() => { ensureMarketGoods() })

    return {
      listId, inputRef, popRef, query, open, activeIndex, activeEntry, view,
      marketGoods, marketGoodsLoading, marketGoodsError,
      optionId, onInput, onFocus, onKeydown, choose, clear, retry, focus,
    }
  },
}
</script>

<style scoped>
.msearch { position: relative; flex-shrink: 0; }
.msearch-row { display: flex; align-items: center; gap: 12px; }
.msearch-field { position: relative; flex: 1; min-width: 0; display: flex; align-items: center; }
.msearch-icon { position: absolute; left: 10px; color: var(--muted); pointer-events: none; }
.msearch-input {
  width: 100%;
  min-width: 0;
  height: 32px;
  padding: 0 32px 0 32px;
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  background: rgba(20, 18, 30, 0.45);
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  outline: none;
  transition: border-color 0.15s, background 0.15s;
}
.msearch-input::placeholder { color: var(--muted); }
.msearch-input:hover { border-color: var(--frame); }
.msearch-input:focus { border-color: var(--cream); background: rgba(20, 18, 30, 0.6); }
.msearch-field--open .msearch-input { border-bottom-left-radius: 2px; border-bottom-right-radius: 2px; }
.msearch-clear {
  position: absolute;
  right: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: rgba(233, 223, 196, 0.1);
  color: var(--text-soft);
  cursor: pointer;
}
.msearch-clear:hover { background: rgba(233, 223, 196, 0.2); color: var(--text); }
.msearch-spinner {
  position: absolute;
  right: 9px;
  width: 14px;
  height: 14px;
  border: 2px solid rgba(233, 223, 196, 0.2);
  border-top-color: var(--cream);
  border-radius: 50%;
  animation: msearch-spin 0.8s linear infinite;
}
@keyframes msearch-spin { to { transform: rotate(360deg); } }

.msearch-pop {
  position: absolute;
  z-index: 20;
  top: calc(100% + 3px);
  left: 0;
  right: 0;
  max-height: min(318px, 70vh);
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 4px 4px;
  border: 1px solid var(--frame);
  border-radius: 4px;
  background: rgba(24, 22, 34, 0.98);
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.55);
}
.msearch-section {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 7px 6px 4px;
  background: rgba(24, 22, 34, 0.98);
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--text-soft);
}
.msearch-section-note {
  font-family: var(--font-body);
  letter-spacing: 0;
  text-transform: none;
  color: var(--muted);
}
.msearch-section-action {
  margin-left: auto;
  padding: 0;
  border: none;
  background: none;
  color: var(--muted);
  font-family: var(--font-body);
  font-size: 11px;
  letter-spacing: 0;
  text-transform: none;
  cursor: pointer;
}
.msearch-section-action:hover { color: var(--text); text-decoration: underline; }

.msearch-opt {
  position: relative;
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 42px;
  padding: 3px 6px;
  border-radius: 3px;
  cursor: pointer;
}
.msearch-opt.is-active { background: var(--surface-hover); }
.msearch-opt.is-active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 6px;
  bottom: 6px;
  width: 2px;
  border-radius: 1px;
  background: var(--cream);
}
.msearch-thumb { display: inline-flex; flex-shrink: 0; }
.msearch-thumb :deep(.item-card--mini .item-card__icon),
.msearch-thumb :deep(.item-card--mini.item-card--cargo .item-card__icon) {
  width: 28px;
  height: 34px;
  flex-basis: 28px;
  border-width: 1px;
  border-radius: 4px;
}
.msearch-opt-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.msearch-opt-name {
  font-size: 13px;
  font-weight: 700;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.msearch-opt-name mark {
  padding: 0 1px;
  border-radius: 2px;
  background: rgba(217, 180, 92, 0.24);
  color: inherit;
}
.msearch-opt-meta { font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.msearch-opt.is-quiet .msearch-thumb { opacity: 0.55; }
.msearch-opt.is-quiet .msearch-opt-name { color: var(--muted-strong); font-weight: 600; }
.msearch-opt-flags { flex-shrink: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 2px; }
.msearch-flag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  font-weight: 700;
  color: var(--muted-strong);
  white-space: nowrap;
}
.msearch-flag::before { content: ''; width: 6px; height: 6px; border-radius: 50%; }
.msearch-flag--sell::before { background: var(--gold); }
.msearch-flag--buy::before { background: var(--accent); }

.msearch-more {
  padding: 7px 6px 3px;
  border-top: 1px solid var(--frame-soft);
  font-size: 11px;
  color: var(--muted);
}
.msearch-message {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  padding: 11px 8px 8px;
  font-size: 12px;
  color: var(--text-soft);
}
.msearch-hint { font-size: 11px; color: var(--muted); line-height: 1.45; }
.msearch-hint b { color: var(--text-soft); font-weight: 700; }
.msearch-retry { min-height: 26px; padding: 3px 10px; font-size: 11px; }
</style>
