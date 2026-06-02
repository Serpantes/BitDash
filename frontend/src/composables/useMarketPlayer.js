import { ref, watch } from 'vue'

const STORAGE_KEY = 'bitdash:market-player'

function load() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const o = JSON.parse(raw)
    if (o?.entityId != null && o?.username) {
      return { entityId: String(o.entityId), username: String(o.username) }
    }
    return null
  } catch {
    return null
  }
}

const selectedPlayer = ref(load())

watch(
  selectedPlayer,
  (v) => {
    if (!v) localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, JSON.stringify(v))
  },
  { deep: true },
)

export function useMarketPlayer() {
  function setPlayer(player) {
    if (!player?.entityId || !player?.username) return
    selectedPlayer.value = {
      entityId: String(player.entityId),
      username: String(player.username),
    }
  }

  function clearPlayer() {
    selectedPlayer.value = null
  }

  return { selectedPlayer, setPlayer, clearPlayer }
}
