/** BitJita `/players/{id}/stats` values[] field order (index = stat). */
export const PLAYER_STAT_FIELDS = [
  'maxHealth',
  'maxStamina',
  'passiveHealthRegenRate',
  'passiveStaminaRegenRate',
  'movementMultiplier',
  'sprintMultiplier',
  'sprintStaminaDrain',
  'armor',
  'cooldownMultiplier',
  'huntingWeaponPower',
  'strength',
  'coldProtection',
  'heatProtection',
  'evasion',
  'toolbeltSlots',
  'craftingSpeed',
  'gatheringSpeed',
  'buildingSpeed',
  'satiationRegenRate',
  'maxSatiation',
  'defenseLevel',
  'forestrySpeed',
  'carpentrySpeed',
  'masonrySpeed',
  'miningSpeed',
  'smithingSpeed',
  'scholarSpeed',
  'leatherworkingSpeed',
  'huntingSpeed',
  'tailoringSpeed',
  'farmingSpeed',
  'fishingSpeed',
  'cookingSpeed',
  'foragingSpeed',
  'forestryPower',
  'carpentryPower',
  'masonryPower',
  'miningPower',
  'smithingPower',
  'scholarPower',
  'leatherworkingPower',
  'huntingPower',
  'tailoringPower',
  'farmingPower',
  'fishingPower',
  'cookingPower',
  'foragingPower',
  'activeHealthRegenRate',
  'activeStaminaRegenRate',
  'climbProficiency',
  'experienceRate',
  'accuracy',
  'maxTeleportationEnergy',
  'teleportationEnergyRegenRate',
  'constructionPower',
]

export function parsePlayerStatsValues(values) {
  if (!values || !Array.isArray(values)) return null
  const stats = {}
  PLAYER_STAT_FIELDS.forEach((key, index) => {
    stats[key] = values[index] ?? 0
  })
  return stats
}

export function characterVitalsFromStats(statsPayload, live = {}) {
  const statValues = live.statsValues ?? statsPayload?.values
  const parsed = parsePlayerStatsValues(statValues)
  if (!parsed) return null

  // Max values come from character stats (REST + character_stats_state), not health/stamina_state.
  // Live health/stamina channels only drive current values.
  const maxHp = Number(parsed.maxHealth) || 100
  const maxStamina = Number(parsed.maxStamina) || 100
  const maxSatiation = Number(parsed.maxSatiation) || 100

  const currentHp = live.currentHealth != null ? Number(live.currentHealth) : maxHp
  const currentStamina = live.currentStamina != null ? Number(live.currentStamina) : maxStamina
  const currentSatiation = live.satiation != null ? Number(live.satiation) : maxSatiation

  return {
    hp: { current: currentHp, max: Math.max(maxHp, currentHp) },
    stamina: { current: currentStamina, max: Math.max(maxStamina, currentStamina) },
    satiation: { current: currentSatiation, max: maxSatiation },
  }
}

export function vitalsBarPercent(bar) {
  if (!bar?.max) return 0
  return Math.min(100, Math.max(0, (bar.current / bar.max) * 100))
}

export function formatVitalsValue(bar) {
  if (!bar) return '—'
  return `${Math.round(bar.current)} / ${Math.round(bar.max)}`
}
