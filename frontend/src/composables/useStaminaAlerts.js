import { watch } from 'vue'
import { playStaminaFullSound, playStaminaLowSound } from '../vitalsSounds'

const FULL_RATIO = 0.995
const LOW_RATIO = 0.02

function isStaminaFull(current, max) {
  if (!max || max <= 0) return false
  return current >= max * FULL_RATIO || current >= max - 0.5
}

function isStaminaLow(current, max) {
  if (!max || max <= 0) return false
  return current <= max * LOW_RATIO || current <= 1
}

/**
 * Stamina sounds: full once on reaching max, low on every live update while depleted.
 * @param {() => Array<{ playerId: string, current: number, max: number }>} getSnapshots
 */
export function useStaminaAlerts(getSnapshots) {
  const track = new Map()

  watch(
    () => getSnapshots(),
    (snapshots) => {
      const seen = new Set()
      snapshots.forEach((snap) => {
        const id = String(snap.playerId)
        seen.add(id)
        const current = Number(snap.current)
        const max = Number(snap.max)
        if (!Number.isFinite(current) || !Number.isFinite(max) || max <= 0) return

        const isFull = isStaminaFull(current, max)
        const isLow = isStaminaLow(current, max)
        const prev = track.get(id)

        if (prev) {
          if (isFull && !prev.isFull) playStaminaFullSound()
          if (isLow && current !== prev.current) playStaminaLowSound()
        }

        track.set(id, { isFull, isLow, current })
      })

      track.forEach((_, id) => {
        if (!seen.has(id)) track.delete(id)
      })
    },
    { deep: true },
  )

  return {
    reset: () => track.clear(),
  }
}
