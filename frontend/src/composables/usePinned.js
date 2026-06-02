import { ref, watch } from 'vue'

const STORAGE_KEY = 'bitdash:pinned'

function load() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  } catch {
    return []
  }
}

// Singleton reactive state
const pinned = ref(load())

watch(pinned, (val) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(val))
}, { deep: true })

export function usePinned() {
  function isPinned(inventoryId) {
    return pinned.value.some((p) => p.inventoryId === inventoryId)
  }

  function pin(player, inv) {
    if (isPinned(inv.entityId)) return
    pinned.value = [
      ...pinned.value,
      {
        inventoryId: inv.entityId,
        playerEntityId: player.entityId,
        playerName: player.username,
        inventoryName: inv.inventoryName,
        regionId: inv.regionId,
        claimName: inv.claimName ?? null,
      },
    ]
  }

  function unpin(inventoryId) {
    pinned.value = pinned.value.filter((p) => p.inventoryId !== inventoryId)
  }

  return { pinned, isPinned, pin, unpin }
}
