import { ref, watch } from 'vue'

const STORAGE_KEY = 'bitdash:pinned'
// The browser's own blocks, kept aside when the server copy replaces them.
const LOCAL_BACKUP_KEY = 'bitdash:pinned:local-backup'
const DASHBOARD_URL = '/api/dashboard'
const SAVE_DELAY_MS = 600
const RETRY_DELAY_MS = 5000

function load() {
  try {
    const value = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    return Array.isArray(value) ? value : []
  } catch {
    return []
  }
}

function storeLocal(json) {
  try {
    localStorage.setItem(STORAGE_KEY, json)
  } catch {
    // Quota or private mode: the server copy is the one that matters.
  }
}

const pinned = ref(load())

// The server copy (DASHBOARD_FILE on the backend) is the source of truth;
// localStorage is only a fast cache. serverRev stays null until the server
// answered once, and nothing is written to the server before that, so a tab
// that has not seen the saved blocks can never overwrite them.
let serverRev = null
let savedJson = null
let saveTimer = null
let saving = false
let saveAgain = false

watch(pinned, (val) => {
  storeLocal(JSON.stringify(val))
  scheduleSave(SAVE_DELAY_MS)
}, { deep: true })

async function fetchDashboard(timeoutMs = 5000) {
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), timeoutMs)
  try {
    const res = await fetch(DASHBOARD_URL, { cache: 'no-store', signal: ctrl.signal })
    if (!res.ok) throw new Error(`dashboard: HTTP ${res.status}`)
    return await res.json()
  } finally {
    clearTimeout(timer)
  }
}

function adoptServerState(state) {
  serverRev = state.rev
  savedJson = JSON.stringify(state.blocks ?? [])
  if (savedJson === JSON.stringify(pinned.value)) return false
  if (pinned.value.length) {
    try { localStorage.setItem(LOCAL_BACKUP_KEY, JSON.stringify(pinned.value)) } catch { /* ignore */ }
  }
  storeLocal(savedJson)
  pinned.value = JSON.parse(savedJson)
  return true
}

// Called once before the app mounts: pull the saved blocks from the server,
// or seed the server from this browser when nothing was saved there yet.
export async function initPinnedSync() {
  let state = null
  for (let attempt = 0; attempt < 4 && !state; attempt += 1) {
    try {
      state = await fetchDashboard()
    } catch (err) {
      if (attempt === 3) console.warn('BitDash: dashboard server unavailable, using this browser\'s copy only', err)
      else await new Promise((r) => setTimeout(r, 400 * 2 ** attempt))
    }
  }
  if (!state) return
  if (state.rev === 0) {
    serverRev = 0
    savedJson = '[]'
    if (pinned.value.length) await flush()
  } else {
    adoptServerState(state)
  }
  document.addEventListener('visibilitychange', onVisibilityChange)
  window.addEventListener('pagehide', flushOnExit)
}

function scheduleSave(delay) {
  if (serverRev === null) return
  clearTimeout(saveTimer)
  saveTimer = setTimeout(flush, delay)
}

async function flush() {
  clearTimeout(saveTimer)
  if (serverRev === null) return
  if (saving) {
    saveAgain = true
    return
  }
  const json = JSON.stringify(pinned.value)
  if (json === savedJson) return
  saving = true
  try {
    const res = await fetch(DASHBOARD_URL, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseRev: serverRev, blocks: pinned.value }),
    })
    if (res.status === 409) {
      // Changed from another tab or device: take that version and start over
      // from it so the dashboard state never forks.
      const state = await res.json()
      adoptServerState(state)
      location.reload()
      return
    }
    if (!res.ok) throw new Error(`dashboard save: HTTP ${res.status}`)
    const state = await res.json()
    serverRev = state.rev
    savedJson = json
  } catch (err) {
    console.warn('BitDash: dashboard save failed, retrying', err)
    saveAgain = false
    scheduleSave(RETRY_DELAY_MS)
  } finally {
    saving = false
    if (saveAgain) {
      saveAgain = false
      flush()
    }
  }
}

function flushOnExit() {
  if (serverRev === null || saving) return
  const json = JSON.stringify(pinned.value)
  if (json === savedJson) return
  fetch(DASHBOARD_URL, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ baseRev: serverRev, blocks: pinned.value }),
    keepalive: json.length < 60000,
  }).catch(() => {})
}

async function onVisibilityChange() {
  if (document.visibilityState === 'hidden') {
    flush()
    return
  }
  if (serverRev === null || saving || JSON.stringify(pinned.value) !== savedJson) return
  try {
    const state = await fetchDashboard()
    if (state.rev !== serverRev && adoptServerState(state)) location.reload()
  } catch {
    // Offline for a moment; the next save will sort it out.
  }
}

export function usePinned() {
  function isPinned(blockId) {
    return pinned.value.some((p) => p.inventoryId === blockId)
  }

  function pin(player, inv) {
    pinBlock({
      sourceType: 'player',
      inventoryId: inv.entityId,
      playerEntityId: player.entityId,
      playerName: player.username,
      inventoryName: inv.inventoryName,
      regionId: inv.regionId,
      claimName: inv.claimName ?? null,
    })
  }

  function pinBlock(block) {
    if (!block?.inventoryId || isPinned(block.inventoryId)) return
    pinned.value = [
      ...pinned.value,
      {
        blockType: 'inventory',
        sourceType: 'player',
        ...block,
      },
    ]
  }

  function unpin(inventoryId) {
    pinned.value = pinned.value.filter((p) => p.inventoryId !== inventoryId)
  }

  function setDashboardLayouts(cols, positions) {
    pinned.value = pinned.value.map((p) => {
      const pos = positions[p.inventoryId]
      if (!pos) return p
      const { dashboardLayout, ...rest } = p
      return {
        ...rest,
        dashboardLayouts: { ...(p.dashboardLayouts ?? {}), [cols]: { x: pos.x, y: pos.y } },
      }
    })
  }

  function updatePinnedBlock(inventoryId, patch) {
    pinned.value = pinned.value.map((p) => {
      if (p.inventoryId !== inventoryId) return p
      return { ...p, ...patch }
    })
  }

  return { pinned, isPinned, pin, pinBlock, unpin, setDashboardLayouts, updatePinnedBlock }
}
