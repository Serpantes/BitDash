export const FOOD_BUFF_TYPE = 'Food Buffs'

export function isFoodBuffType(buff) {
  const type = buff?.buffTypeName ?? buff?.buffName ?? ''
  return type === FOOD_BUFF_TYPE
}

export function formatBuffTimeRemaining(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'
  const total = Math.floor(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hours > 0) return `${hours}ч ${minutes}м`
  if (minutes > 0) return `${minutes}м ${secs}с`
  return `${secs}с`
}

export function normalizeActiveBuff(buff) {
  const name = buff?.buffTypeName ?? buff?.buffName ?? 'Buff'
  const timeRemaining = Number(buff?.timeRemaining) || 0
  return {
    id: buff?.buffId ?? `${name}-${buff?.buffStartTimestamp ?? 0}`,
    name,
    description: buff?.description ?? buff?.buffName ?? name,
    timeRemaining,
    timeLabel: formatBuffTimeRemaining(timeRemaining),
    beneficial: Boolean(buff?.beneficial),
    isFoodBuff: isFoodBuffType(buff),
  }
}

export function activeBuffsFromPayload(payload) {
  const list = Array.isArray(payload) ? payload : (payload?.buffs ?? [])
  return list
    .filter((buff) => buff?.status === 'active')
    .map(normalizeActiveBuff)
    .sort((a, b) => b.timeRemaining - a.timeRemaining)
}

export function hasActiveFoodBuff(payload) {
  return activeBuffsFromPayload(payload).some((buff) => buff.isFoodBuff)
}
