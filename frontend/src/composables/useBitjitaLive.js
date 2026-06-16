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

const subscribedChannels = new Set()
const subscribedPlayers = new Set()
let ws = null
let reconnectTimer = null
let reconnectAttempts = 0
let visibilityListenerBound = false

function playerChannels(playerId) {
  const id = String(playerId)
  return [
    `health_state:${id}`,
    `stamina_state:${id}`,
    `satiation_state:${id}`,
    `character_stats_state:${id}`,
  ]
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
      default:
        break
    }
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return
  if (subscribedPlayers.size === 0) return
  const delay = Math.min(30000, 1000 * (reconnectAttempts + 1))
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    reconnectAttempts += 1
    connect()
  }, delay)
}

function bindVisibilityReconnect() {
  if (visibilityListenerBound || typeof document === 'undefined') return
  visibilityListenerBound = true
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible' || subscribedPlayers.size === 0) return
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      reconnectAttempts = 0
      connect()
    }
  })
}

function connect() {
  if (typeof WebSocket === 'undefined') return
  if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) return

  bindVisibilityReconnect()
  ws = new WebSocket(liveWebSocketUrl())

  ws.onopen = () => {
    reconnectAttempts = 0
    if (subscribedChannels.size > 0) {
      sendJson({ type: 'subscribe', channels: [...subscribedChannels] })
    }
  }

  ws.onmessage = (event) => {
    reconnectAttempts = 0
    try {
      handleMessage(event.data)
    } catch {
      // ignore malformed payloads
    }
  }

  ws.onclose = () => {
    ws = null
    scheduleReconnect()
  }

  ws.onerror = () => {
    if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) {
      ws.close()
    }
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
  if (next.size === 0 && ws) {
    ws.close()
    ws = null
  }
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
    subscribePlayer,
    unsubscribePlayer,
    syncPlayerSubscriptions,
    liveSnapshot,
  }
}
