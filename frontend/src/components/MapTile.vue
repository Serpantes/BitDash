<template>
  <div class="map-tile" @keydown.esc="closeOverlays">
    <div class="map-toolbar">
      <label class="bc-search map-search">
        <AppIcon name="search" :size="14" />
        <input
          v-model="query"
          class="bc-input"
          type="text"
          :placeholder="t('map.searchPlaceholder')"
          :disabled="!ready"
          @focus="resultsOpen = true"
          @keydown.down.prevent="moveActive(1)"
          @keydown.up.prevent="moveActive(-1)"
          @keydown.enter.prevent="pickActive"
        />
        <button v-if="query" type="button" class="map-search-clear" :title="t('common.clear')" @click="clearSearch">
          <AppIcon name="close" :size="12" />
        </button>
      </label>
      <button
        type="button"
        class="bc-icon-btn map-layers-btn"
        :class="{ active: layersOpen }"
        :title="t('map.layers')"
        :disabled="!ready"
        @click="toggleLayers"
      ><AppIcon name="layers" /></button>
    </div>

    <div class="map-stage">
      <iframe
        ref="frame"
        class="map-frame"
        :src="initialSrc"
        :title="t('map.frameTitle')"
        referrerpolicy="no-referrer"
        @load="onFrameLoad"
      />

      <div v-if="!ready" class="map-veil">
        <span v-if="failed">{{ t('map.notResponding') }} · <button type="button" class="map-link" @click="reload">{{ t('map.reload') }}</button></span>
        <span v-else>{{ t('map.loading') }}</span>
      </div>

      <div v-if="resultsOpen && searchActive" class="map-panel map-results" role="listbox">
        <template v-for="group in results.groups" :key="group.title">
          <h3 class="map-panel-title">{{ mapLabel(group.title) }}</h3>
          <button
            v-for="entry in group.items"
            :key="entry.key"
            type="button"
            role="option"
            class="map-result"
            :class="{ active: entry.index === activeIndex }"
            :aria-selected="entry.index === activeIndex"
            @mouseenter="activeIndex = entry.index"
            @click="pick(entry)"
          >
            <span class="map-result-swatch" :style="{ background: entry.color || 'var(--border-strong)' }"></span>
            <span class="map-result-label">{{ entry.label }}</span>
            <span class="map-result-meta">{{ entry.metaText }}</span>
          </button>
        </template>
        <div v-if="!results.groups.length" class="bc-muted-note map-results-empty">
          {{ results.done ? t('common.nothingFound') : t('common.searching') }}
        </div>
      </div>

      <div v-if="layersOpen" class="map-panel map-layers">
        <div v-if="!controls.length" class="bc-muted-note">{{ t('common.loading') }}</div>
        <template v-else>
          <section v-if="shaped.baseMaps.length" class="map-layers-section">
            <h3 class="map-panel-title">{{ t('map.baseMap') }}</h3>
            <div class="bc-seg">
              <button
                v-for="base in shaped.baseMaps"
                :key="base.id"
                type="button"
                class="bc-seg-btn"
                :class="{ active: base.checked }"
                @click="!base.checked && toggle(base)"
              >{{ mapLabel(base.label) }}</button>
            </div>
          </section>
          <section v-if="shaped.overlays.length || shaped.players.length" class="map-layers-section">
            <h3 class="map-panel-title">{{ t('map.overlays') }}</h3>
            <button v-for="item in [...shaped.overlays, ...shaped.players]" :key="item.id" type="button" class="map-toggle-row" @click="toggle(item)">
              <span>{{ mapLabel(item.label) }}</span>
              <span class="bc-toggle" :class="{ on: item.checked }"></span>
            </button>
          </section>
          <section v-for="group in shaped.groups" :key="group.item.id" class="map-layers-section">
            <button type="button" class="map-toggle-row map-toggle-row--group" @click="toggle(group.item)">
              <span class="map-panel-title">{{ mapLabel(group.item.label) }}</span>
              <span class="bc-toggle" :class="{ on: group.item.checked, mixed: group.item.indeterminate }"></span>
            </button>
            <button v-for="item in group.named" :key="item.id" type="button" class="map-toggle-row" @click="toggle(item)">
              <span>{{ mapLabel(item.label) }}</span>
              <span class="bc-toggle" :class="{ on: item.checked }"></span>
            </button>
            <div v-if="group.tiers.length" class="map-tiers">
              <button
                v-for="item in group.tiers"
                :key="item.id"
                type="button"
                class="map-tier"
                :class="{ active: item.checked }"
                :title="`${mapLabel(group.item.label)} ${mapLabel(item.label)}`"
                @click="toggle(item)"
              >{{ mapLabel(item.label) }}</button>
            </div>
          </section>
        </template>
      </div>

      <div class="map-zoom">
        <button type="button" class="map-zoom-btn" :title="t('map.zoomIn')" :disabled="!ready" @click="post('zoomIn')"><AppIcon name="plus" :size="14" /></button>
        <button type="button" class="map-zoom-btn" :title="t('map.zoomOut')" :disabled="!ready" @click="post('zoomOut')"><AppIcon name="minus" :size="14" /></button>
        <button type="button" class="map-zoom-btn" :title="t('map.reset')" :disabled="!ready" @click="post('reset')"><AppIcon name="home" :size="14" /></button>
      </div>

      <div v-if="tracked.length" class="map-tracked">
        <span
          v-for="item in tracked"
          :key="item.label"
          class="map-chip"
          :class="{ 'map-chip--hidden': !item.visible }"
        >
          <button
            type="button"
            class="map-chip-label"
            :title="item.visible ? t('map.hide') : t('map.show')"
            @click="post('showTracked', { label: item.label, visible: !item.visible })"
          >
            <AppIcon v-if="item.kind === 'player'" name="player" :size="12" :style="{ color: item.color || 'var(--cream)' }" />
            <span v-else class="map-result-swatch" :style="{ background: item.color || 'var(--cream)' }"></span>
            {{ trackedLabel(item.label) }}
          </button>
          <button type="button" class="map-chip-remove" :title="t('map.remove')" @click="post('untrack', { label: item.label })">
            <AppIcon name="close" :size="11" />
          </button>
        </span>
      </div>

      <div v-if="ready && relayState === 'reconnecting'" class="map-relay">{{ t('map.reconnecting') }}</div>

      <div v-if="coords" class="map-coords">N {{ coords.n }} · E {{ coords.e }}</div>
    </div>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
import {
  controlStates,
  controlsDiffer,
  mapEmbedSrc,
  mapLabel,
  shapeControls,
  trackedLabel,
  trackingQuery,
} from '../mapControls'

const TAG = 'bitdash-map'
const SEARCH_DEBOUNCE_MS = 250
const SAVE_DEBOUNCE_MS = 600
const READY_TIMEOUT_MS = 25000
// the bridge makes upstream retry its relay socket; if that keeps failing, start over
const RELAY_RELOAD_MS = 90000

export default {
  name: 'MapTile',
  components: { AppIcon },
  props: {
    mapState: { type: Object, default: null },
    legacyUrl: { type: String, default: null },
  },
  emits: ['update'],
  setup(props, { emit }) {
    const frame = ref(null)
    const initialSrc = ref(mapEmbedSrc(props.mapState, props.legacyUrl))
    const ready = ref(false)
    const failed = ref(false)
    const query = ref('')
    const resultsOpen = ref(false)
    const layersOpen = ref(false)
    const results = reactive({ seq: 0, groups: [], done: false })
    const activeIndex = ref(0)
    const controls = ref([])
    const tracked = ref([])
    const cursor = ref(null)
    const center = ref(props.mapState?.view ?? null)
    const relayState = ref('')
    let relayTimer = null
    let searchSeq = 0
    let searchTimer = null
    let saveTimer = null
    let readyTimer = null
    let controlsRestored = false
    const pending = { ...(props.mapState ?? {}) }

    const searchActive = computed(() => query.value.trim().length >= 2)
    const shaped = computed(() => shapeControls(controls.value))
    const coords = computed(() => cursor.value ?? center.value)
    const indexedGroups = computed(() => {
      let index = 0
      return results.groups.map((group) => ({
        title: group.title,
        items: group.items.map((item, position) => {
          const metaText = item.n != null ? `N ${item.n} · E ${item.e}` : item.meta
          return { ...item, metaText, position, group: group.title, index: index++, key: `${group.title}:${item.label}:${index}` }
        }),
      }))
    })
    const flatResults = computed(() => indexedGroups.value.flatMap((group) => group.items))

    function post(cmd, args = {}) {
      const payload = { target: TAG, cmd, args: JSON.parse(JSON.stringify(args)) }
      frame.value?.contentWindow?.postMessage(payload, window.location.origin)
    }
    function save(patch) {
      Object.assign(pending, patch)
      clearTimeout(saveTimer)
      saveTimer = setTimeout(() => emit('update', { mapState: { ...pending } }), SAVE_DEBOUNCE_MS)
    }

    function setRelayState(state) {
      relayState.value = state ?? ''
      clearTimeout(relayTimer)
      relayTimer = null
      if (relayState.value === 'reconnecting') relayTimer = setTimeout(reloadIfVisible, RELAY_RELOAD_MS)
    }
    function reloadIfVisible() {
      if (relayState.value !== 'reconnecting') return
      if (document.visibilityState === 'visible') reload()
      else relayTimer = setTimeout(reloadIfVisible, 5000)
    }
    function onMessage(event) {
      if (event.source !== frame.value?.contentWindow || event.origin !== window.location.origin) return
      const msg = event.data
      if (!msg || msg.source !== TAG) return
      const data = msg.data
      switch (msg.type) {
        case 'ready':
          ready.value = true
          failed.value = false
          clearTimeout(readyTimer)
          center.value = data.view
          setRelayState(data.relay)
          post('controls')
          post('tracked')
          break
        case 'view':
          center.value = data
          save({ view: data })
          break
        case 'cursor':
          cursor.value = data
          break
        case 'query':
          save({ query: trackingQuery(data.query) })
          break
        case 'results':
          if (data.seq !== searchSeq) return
          results.groups = data.groups ?? []
          results.done = Boolean(data.done)
          if (activeIndex.value >= flatResults.value.length) activeIndex.value = 0
          break
        case 'controls':
          controls.value = data.items ?? []
          if (!controlsRestored) {
            controlsRestored = true
            if (controlsDiffer(controls.value, props.mapState?.controls)) {
              post('setControls', { states: props.mapState.controls })
              return
            }
          }
          save({ controls: controlStates(controls.value) })
          break
        case 'tracked':
          tracked.value = data.items ?? []
          break
        case 'relay':
          setRelayState(data.state)
          break
        default:
      }
    }

    function onFrameLoad() {
      ready.value = false
      setRelayState('')
      controlsRestored = false
      clearTimeout(readyTimer)
      readyTimer = setTimeout(() => { if (!ready.value) failed.value = true }, READY_TIMEOUT_MS)
      post('hello')
    }
    function reload() {
      failed.value = false
      initialSrc.value = `${mapEmbedSrc(pending, props.legacyUrl)}&reload=${Date.now()}`
    }

    watch(query, (value) => {
      clearTimeout(searchTimer)
      const q = value.trim()
      activeIndex.value = 0
      if (q.length < 2) {
        results.groups = []
        results.done = false
        post('clearSearch')
        return
      }
      resultsOpen.value = true
      layersOpen.value = false
      searchTimer = setTimeout(() => {
        searchSeq += 1
        results.groups = []
        results.done = false
        post('search', { q, seq: searchSeq })
      }, SEARCH_DEBOUNCE_MS)
    })
    function moveActive(step) {
      const count = flatResults.value.length
      if (!count) return
      resultsOpen.value = true
      activeIndex.value = (activeIndex.value + step + count) % count
    }
    function pickActive() {
      const entry = flatResults.value[activeIndex.value]
      if (entry) pick(entry)
    }
    function pick(entry) {
      post('pick', { group: entry.group, label: entry.label, meta: entry.meta, n: entry.n, e: entry.e, position: entry.position })
      resultsOpen.value = false
      query.value = ''
    }
    function clearSearch() {
      query.value = ''
      resultsOpen.value = false
    }
    function toggleLayers() {
      layersOpen.value = !layersOpen.value
      if (layersOpen.value) {
        resultsOpen.value = false
        post('controls')
      }
    }
    function toggle(item) {
      post('toggle', { id: item.id })
    }
    function closeOverlays() {
      resultsOpen.value = false
      layersOpen.value = false
    }
    function onWindowBlur() {
      if (document.activeElement === frame.value) closeOverlays()
    }

    onMounted(() => {
      window.addEventListener('message', onMessage)
      window.addEventListener('blur', onWindowBlur)
    })
    onUnmounted(() => {
      window.removeEventListener('message', onMessage)
      window.removeEventListener('blur', onWindowBlur)
      clearTimeout(searchTimer)
      clearTimeout(readyTimer)
      clearTimeout(relayTimer)
      if (saveTimer) {
        clearTimeout(saveTimer)
        emit('update', { mapState: { ...pending } })
      }
    })

    return {
      frame, initialSrc, ready, failed, query, resultsOpen, layersOpen, results: computed(() => ({ groups: indexedGroups.value, done: results.done })),
      activeIndex, controls, shaped, tracked, coords, searchActive, relayState,
      post, onFrameLoad, reload, moveActive, pickActive, pick, clearSearch, toggleLayers, toggle, closeOverlays,
      mapLabel, trackedLabel,
    }
  },
}
</script>

<style scoped>
.map-tile { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 8px; }
.map-toolbar { display: flex; align-items: center; gap: 6px; }
.map-search { flex: 1; }
.map-search .bc-input { height: 30px; padding-right: 30px; font-size: 12px; }
.map-search-clear {
  position: absolute;
  right: 6px;
  display: inline-flex;
  padding: 4px;
  border: none;
  border-radius: 3px;
  background: none;
  color: var(--muted);
  cursor: pointer;
}
.map-search-clear:hover { color: var(--text); }
.map-layers-btn { width: 30px; height: 30px; border-color: var(--border-strong); }
.map-layers-btn.active { background: var(--cream); border-color: var(--cream); color: var(--on-cream); }

.map-stage {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--border-strong);
  border-radius: 3px;
  background: #15131d;
}
.map-frame { display: block; width: 100%; height: 100%; border: none; }
.map-veil {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(21, 19, 29, 0.85);
  color: var(--muted);
  font-size: 12px;
}
.map-link { padding: 0; border: none; background: none; color: var(--cream); font: inherit; text-decoration: underline; cursor: pointer; }

.map-panel {
  position: absolute;
  z-index: 4;
  top: 6px;
  max-height: calc(100% - 12px);
  overflow-y: auto;
  padding: 6px;
  border: 1px solid var(--frame);
  border-radius: 4px;
  background: rgba(24, 22, 34, 0.97);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.55);
}
.map-results { left: 6px; right: 6px; max-width: 420px; }
.map-layers { right: 40px; width: min(280px, calc(100% - 46px)); }
.map-panel-title {
  margin: 6px 6px 3px;
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--gold);
}
.map-result {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 7px;
  border: none;
  border-radius: 3px;
  background: none;
  color: var(--text);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}
.map-result.active { background: rgba(233, 223, 196, 0.1); }
.map-result-swatch { width: 8px; height: 8px; flex-shrink: 0; transform: rotate(45deg); border-radius: 1px; }
.map-result-label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.map-result-meta { flex-shrink: 0; color: var(--muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.map-results-empty { padding: 8px; }

.map-layers-section + .map-layers-section { margin-top: 4px; padding-top: 4px; border-top: 1px solid rgba(233, 223, 196, 0.08); }
.map-layers-section > .bc-seg { margin: 2px 6px 4px; }
.map-toggle-row {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 4px 6px;
  border: none;
  border-radius: 3px;
  background: none;
  color: var(--text-soft);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}
.map-toggle-row:hover { background: rgba(233, 223, 196, 0.06); color: var(--text); }
.map-toggle-row--group .map-panel-title { margin: 0; }
.map-tiers { display: flex; flex-wrap: wrap; gap: 3px; padding: 2px 6px 4px; }
.map-tier {
  min-width: 30px;
  padding: 2px 5px;
  border: 1px solid var(--border-strong);
  border-radius: 3px;
  background: none;
  color: var(--muted-strong);
  font-size: 10px;
  font-weight: 800;
  cursor: pointer;
}
.map-tier.active { border-color: var(--cream); background: var(--cream); color: var(--on-cream); }

.map-zoom { position: absolute; z-index: 3; right: 6px; bottom: 30px; display: flex; flex-direction: column; gap: 3px; }
.map-zoom-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid var(--frame);
  border-radius: 3px;
  background: rgba(24, 22, 34, 0.9);
  color: var(--text);
  cursor: pointer;
}
.map-zoom-btn:hover:not(:disabled) { background: var(--cream); color: var(--on-cream); }
.map-zoom-btn:disabled { opacity: 0.4; cursor: default; }

.map-tracked {
  position: absolute;
  z-index: 3;
  left: 6px;
  right: 44px;
  bottom: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  pointer-events: none;
}
.map-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  border: 1px solid var(--frame);
  border-radius: 3px;
  background: rgba(24, 22, 34, 0.92);
  pointer-events: auto;
}
.map-chip--hidden { opacity: 0.55; border-style: dashed; }
.map-chip-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 3px 4px 3px 7px;
  border: none;
  background: none;
  color: var(--text);
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}
.map-chip-remove {
  display: inline-flex;
  padding: 4px 5px;
  border: none;
  border-left: 1px solid rgba(233, 223, 196, 0.15);
  background: none;
  color: var(--muted);
  cursor: pointer;
}
.map-chip-remove:hover { color: var(--danger); }
.map-relay {
  position: absolute;
  z-index: 2;
  left: 50%;
  top: 8px;
  transform: translateX(-50%);
  padding: 3px 10px;
  border-radius: 3px;
  background: rgba(24, 22, 34, 0.9);
  border: 1px solid var(--border-strong);
  color: var(--text-soft);
  font-size: 11px;
  font-weight: 700;
  pointer-events: none;
}
.map-coords {
  position: absolute;
  z-index: 2;
  right: 6px;
  bottom: 6px;
  padding: 2px 6px;
  border-radius: 3px;
  background: rgba(24, 22, 34, 0.85);
  color: var(--text-soft);
  font-size: 10px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
}
</style>
