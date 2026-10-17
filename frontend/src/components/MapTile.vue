<template>
  <div class="map-tile">
    <div class="map-stage">
      <iframe
        ref="frame"
        class="map-frame"
        :src="src"
        :title="t('map.frameTitle')"
        referrerpolicy="no-referrer"
        @load="onFrameLoad"
      />

      <div v-if="!ready" class="map-veil">
        <span v-if="failed">{{ t('map.notResponding') }} · <button type="button" class="map-link" @click="reload">{{ t('map.reload') }}</button></span>
        <span v-else>{{ t('map.loading') }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { onMounted, onUnmounted, ref } from 'vue'
import { mapEmbedSrc } from '../mapControls'

const TAG = 'bitdash-map'
const SAVE_DEBOUNCE_MS = 600
const READY_TIMEOUT_MS = 30000

export default {
  name: 'MapTile',
  props: {
    mapState: { type: Object, default: null },
    legacyUrl: { type: String, default: null },
  },
  emits: ['update'],
  setup(props, { emit }) {
    const frame = ref(null)
    const src = ref(mapEmbedSrc(props.mapState, props.legacyUrl))
    const ready = ref(false)
    const failed = ref(false)
    let saveTimer = null
    let readyTimer = null
    let savedHash = props.mapState?.hash ?? null

    function post(cmd, args = {}) {
      frame.value?.contentWindow?.postMessage({ target: TAG, cmd, args }, window.location.origin)
    }
    // the map's hash is its whole state (view, layers, tracked players and resources)
    function saveHash(hash) {
      if (hash === savedHash) return
      savedHash = hash
      clearTimeout(saveTimer)
      saveTimer = setTimeout(() => {
        saveTimer = null
        emit('update', { mapState: { hash } })
      }, SAVE_DEBOUNCE_MS)
    }

    function onMessage(event) {
      if (event.source !== frame.value?.contentWindow || event.origin !== window.location.origin) return
      const msg = event.data
      if (!msg || msg.source !== TAG) return
      if (msg.type === 'ready') {
        ready.value = true
        failed.value = false
        clearTimeout(readyTimer)
        saveHash(msg.data?.hash ?? '')
      } else if (msg.type === 'hash') {
        saveHash(msg.data?.hash ?? '')
      }
    }

    function onFrameLoad() {
      ready.value = false
      clearTimeout(readyTimer)
      readyTimer = setTimeout(() => { if (!ready.value) failed.value = true }, READY_TIMEOUT_MS)
      post('hello')
    }
    function reload() {
      failed.value = false
      src.value = mapEmbedSrc(savedHash != null ? { hash: savedHash } : props.mapState, props.legacyUrl, Date.now())
    }

    onMounted(() => window.addEventListener('message', onMessage))
    onUnmounted(() => {
      window.removeEventListener('message', onMessage)
      clearTimeout(readyTimer)
      if (saveTimer) {
        clearTimeout(saveTimer)
        emit('update', { mapState: { hash: savedHash } })
      }
    })

    return { frame, src, ready, failed, onFrameLoad, reload }
  },
}
</script>

<style scoped>
.map-tile { flex: 1; min-height: 0; display: flex; flex-direction: column; }
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
</style>
