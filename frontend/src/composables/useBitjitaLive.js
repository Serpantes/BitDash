import { reactive } from 'vue'

function liveWebSocketUrl() {
  if (typeof window === 'undefined') return 'wss://live.bitjita.com'
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/api/live-ws`
}

const healthByPlayer = reactive({})
const staminaByPlayer = reactive({})
const satiationByPlayer = reactive({})
const characterStatsByPlayer = reactive({})
const experienceByPlayer = reactive({})

const subscribedChannels = new Set()
const subscribedPlayers = new Set()
const experiencePlayers = new Set()
const PROBE_IDLE_MS = 45000
const PROBE_TIMEOUT_MS = 10000
const WATCH_TICK_MS = 5000
const OPEN_TIMEOUT_MS = 15000
let ws = null
let reconnectTimer = null
let reconnectAttempts = 0
let watchTimer = null
let lastMessageAt = 0
let lastTickAt = 0
let openedAt = 0
let probeSentAt = 0
let lifecycleBound = false

function playerChannels(playerId) {
  const id = String(playerId)
  return [
    `health_state:${id}`,
    `stamina_state:${id}`,
    `satiation_state:${id}`,
    `character_stats_state:${id}`,
  ]
}

function experienceChannel(playerId) {
  return `experience_state:${playerId}`
}

function sendJson(payload) {
  if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(payload))
}

function subscribeChannels(channels) {
  if (!channels.length) return
  channels.forEach((ch) => subscribedChannels.add(ch))
  sendJson({ type: 'subscribe', channels })
}

function unsubscribeChannels(channels) {
  if (!channels.length) return
  channels.forEach((ch) => subscribedChannels.delete(ch))
  sendJson({ type: 'unsubscribe', channels })
}

function updateHealth(playerId, data) {
  healthByPlayer[String(playerId)] = {
    currentHealth: data.health ?? 0,
    maxHealth: data.max_health ?? 100,
  }
}

function updateStamina(playerId, data) {
  staminaByPlayer[String(playerId)] = {
    currentStamina: data.stamina ?? 0,
    maxStamina: data.max_stamina ?? 100,
  }
}

function updateSatiation(playerId, data) {
  satiationByPlayer[String(playerId)] = {
    satiation: data.satiation ?? 0,
  }
}

function updateCharacterStats(playerId, data) {
  if (!Array.isArray(data?.values)) return
  characterStatsByPlayer[String(playerId)] = {
    values: data.values,
    regionId: data.region_id != null ? Number(data.region_id) : undefined,
    updatedAt: data.timestamp ?? Date.now(),
  }
}

function updateExperience(playerId, data) {
  if (!Array.isArray(data?.experience_stacks)) return
  const xp = {}
  data.experience_stacks.forEach((stack) => {
    xp[String(stack.skill_id)] = Number(stack.quantity) || 0
  })
  experienceByPlayer[String(playerId)] = { xp, updatedAt: Date.now() }
}

function handleMessage(raw) {
  let text = raw
  if (typeof text === 'string' && text.includes('"bitmap"')) {
    text = text.replace(/"bitmap":\s*\[([^\]]*)\]/, (match, inner) => {
      const normalized = inner.split(',').map((part) => `"${part.trim()}"`).join(',')
      return `"bitmap":[${normalized}]`
    })
  }
  const msg = JSON.parse(text)
  if (msg.type === 'event' && msg.channel && msg.data) {
    const [kind, playerId] = String(msg.channel).split(':')
    switch (kind) {
      case 'health_state':
        updateHealth(playerId, msg.data)
        break
      case 'stamina_state':
        updateStamina(playerId, msg.data)
        break
      case 'satiation_state':
        updateSatiation(playerId, msg.data)
        break
      case 'character_stats':
      case 'character_stats_state':
        updateCharacterStats(playerId, msg.data)
        break
      case 'experience_state':
        updateExperience(playerId, msg.data)
        break
      default:
        break
    }
  }
}

function scheduleReconnect() {
  if (reconnectTimer || subscribedChannels.size === 0) return
  const delay = Math.min(30000, 1000 * (reconnectAttempts + 1))
  reconnectAttempts += 1
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, delay)
}

function dropSocket() {
  const socket = ws
  ws = null
  probeSentAt = 0
  if (socket && socket.readyState !== WebSocket.CLOSED) socket.close()
}

// A socket can stay OPEN long after the link died (sleep, network switch, dead NAT entry);
// an empty subscribe is answered by upstream, so silence after one means reconnect.
function probe() {
  if (ws?.readyState !== WebSocket.OPEN || probeSentAt) return
  probeSentAt = Date.now()
  ws.send(JSON.stringify({ type: 'subscribe', channels: [] }))
}

function reviveNow() {
  if (subscribedChannels.size === 0) return
  if (ws?.readyState === WebSocket.OPEN) {
    probeSentAt = 0
    probe()
    return
  }
  if (ws?.readyState === WebSocket.CONNECTING) return
  clearTimeout(reconnectTimer)
  reconnectTimer = null
  reconnectAttempts = 0
  connect()
}

function watch() {
  const now = Date.now()
  const slept = lastTickAt && now - lastTickAt > WATCH_TICK_MS * 4
  lastTickAt = now
  if (subscribedChannels.size === 0) return
  if (!ws) {
    if (!reconnectTimer || slept) reviveNow()
    return
  }
  if (ws.readyState === WebSocket.CONNECTING) {
    if (now - openedAt > OPEN_TIMEOUT_MS) dropSocket()
    return
  }
  if (ws.readyState !== WebSocket.OPEN) return
  if (probeSentAt && now - probeSentAt > PROBE_TIMEOUT_MS) {
    dropSocket()
    scheduleReconnect()
    return
  }
  if (slept) {
    probeSentAt = 0
    probe()
  } else if (now - lastMessageAt > PROBE_IDLE_MS) {
    probe()
  }
}

function bindLifecycle() {
  if (lifecycleBound || typeof window === 'undefined') return
  lifecycleBound = true
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') reviveNow()
  })
  window.addEventListener('online', reviveNow)
  window.addEventListener('pageshow', reviveNow)
}

function startWatch() {
  if (watchTimer) return
  lastTickAt = Date.now()
  watchTimer = setInterval(watch, WATCH_TICK_MS)
}

function stopWatch() {
  clearInterval(watchTimer)
  watchTimer = null
  clearTimeout(reconnectTimer)
  reconnectTimer = null
}

function connect() {
  if (typeof WebSocket === 'undefined' || subscribedChannels.size === 0) return
  if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) return

  bindLifecycle()
  startWatch()
  const socket = new WebSocket(liveWebSocketUrl())
  ws = socket
  openedAt = Date.now()
  probeSentAt = 0

  socket.onopen = () => {
    if (ws !== socket) return
    lastMessageAt = Date.now()
    if (subscribedChannels.size > 0) {
      sendJson({ type: 'subscribe', channels: [...subscribedChannels] })
    }
  }

  socket.onmessage = (event) => {
    if (ws !== socket) return
    lastMessageAt = Date.now()
    probeSentAt = 0
    reconnectAttempts = 0
    try {
      handleMessage(event.data)
    } catch {
    }
  }

  socket.onclose = () => {
    if (ws !== socket) return
    ws = null
    scheduleReconnect()
  }

  socket.onerror = () => {
    if (ws === socket && socket.readyState !== WebSocket.CLOSED) socket.close()
  }
}

function subscribePlayer(playerId) {
  const id = String(playerId)
  if (subscribedPlayers.has(id)) return
  subscribedPlayers.add(id)
  subscribeChannels(playerChannels(id))
  connect()
}

function unsubscribePlayer(playerId) {
  const id = String(playerId)
  if (!subscribedPlayers.has(id)) return
  subscribedPlayers.delete(id)
  unsubscribeChannels(playerChannels(id))
  delete healthByPlayer[id]
  delete staminaByPlayer[id]
  delete satiationByPlayer[id]
  delete characterStatsByPlayer[id]
}

function syncPlayerSubscriptions(playerIds) {
  const next = new Set(playerIds.map((id) => String(id)))
  subscribedPlayers.forEach((id) => {
    if (!next.has(id)) unsubscribePlayer(id)
  })
  next.forEach((id) => subscribePlayer(id))
  closeIfIdle()
}

function syncExperienceSubscriptions(playerIds) {
  const next = new Set(playerIds.map((id) => String(id)))
  const removed = [...experiencePlayers].filter((id) => !next.has(id))
  const added = [...next].filter((id) => !experiencePlayers.has(id))
  removed.forEach((id) => {
    experiencePlayers.delete(id)
    delete experienceByPlayer[id]
  })
  added.forEach((id) => experiencePlayers.add(id))
  unsubscribeChannels(removed.map(experienceChannel))
  subscribeChannels(added.map(experienceChannel))
  if (added.length) connect()
  closeIfIdle()
}

function closeIfIdle() {
  if (subscribedChannels.size > 0) return
  stopWatch()
  dropSocket()
}

function liveSnapshot(playerId) {
  const id = String(playerId)
  const health = healthByPlayer[id]
  const stamina = staminaByPlayer[id]
  const satiation = satiationByPlayer[id]
  const characterStats = characterStatsByPlayer[id]
  return {
    currentHealth: health?.currentHealth,
    currentStamina: stamina?.currentStamina,
    satiation: satiation?.satiation,
    statsValues: characterStats?.values,
  }
}

export function useBitjitaLive() {
  return {
    healthByPlayer,
    staminaByPlayer,
    satiationByPlayer,
    characterStatsByPlayer,
    experienceByPlayer,
    subscribePlayer,
    unsubscribePlayer,
    syncPlayerSubscriptions,
    syncExperienceSubscriptions,
    liveSnapshot,
  }
}
