import { ref, watch } from 'vue'

const STORAGE_KEY = 'bitdash:pinned'

function load() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  } catch {
    return []
  }
}

const pinned = ref(load())

watch(pinned, (val) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(val))
}, { deep: true })

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

  function updatePinnedLayout(inventoryId, layout) {
    pinned.value = pinned.value.map((p) => {
      if (p.inventoryId !== inventoryId) return p
      return {
        ...p,
        dashboardLayout: {
          ...(p.dashboardLayout ?? {}),
          ...layout,
        },
      }
    })
  }

  function updatePinnedBlock(inventoryId, patch) {
    pinned.value = pinned.value.map((p) => {
      if (p.inventoryId !== inventoryId) return p
      return { ...p, ...patch }
    })
  }

  return { pinned, isPinned, pin, pinBlock, unpin, updatePinnedLayout, updatePinnedBlock }
}
