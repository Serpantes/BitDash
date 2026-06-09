export const TIER_COLORS = {
  1: '#636a74',
  2: '#865f45',
  3: '#5c6f4d',
  4: '#49619b',
  5: '#814f86',
  6: '#973a44',
  7: '#927014',
  8: '#538484',
  9: '#474953',
  10: '#97aebd',
}

export const RARITY_COLORS = {
  default: '#6a687e',
  common: '#706048',
  rare: '#384860',
  uncommon: '#684838',
  mythic: '#2632e8',
  legendary: '#0e7490',
  epic: '#a16207',
}

const ICON_CACHE_VERSION = '2026-06-09-2'

export function itemInitials(name) {
  return String(name || '?')
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || '?'
}

export function iconUrl(iconAssetName) {
  if (!iconAssetName) return null
  const params = new URLSearchParams({ iconAssetName, v: ICON_CACHE_VERSION })
  return `/api/bitjita-icon?${params.toString()}`
}

export function tierNumber(tier) {
  const value = Number(tier)
  return Number.isFinite(value) && value > 0 ? value : null
}

export function hexToRgb(hex) {
  const clean = String(hex || '').replace('#', '')
  if (clean.length !== 6) return null
  const value = Number.parseInt(clean, 16)
  if (!Number.isFinite(value)) return null
  return {
    r: (value >> 16) & 255,
    g: (value >> 8) & 255,
    b: value & 255,
  }
}

export function tierStyle(tier) {
  const normalized = tierNumber(tier)
  const color = normalized ? TIER_COLORS[normalized] : null
  if (!color) return null
  const rgb = hexToRgb(color)
  return {
    '--tier-color': color,
    '--tier-bg': rgb ? `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.18)` : `${color}2e`,
    '--tier-fg': '#171625',
  }
}

export function rarityStyle(rarity) {
  const key = String(rarity || '').trim().toLowerCase()
  const color = RARITY_COLORS[key]
  if (!color) return null
  return { '--rarity-color': color }
}

export function itemTypeKind(itemType) {
  return itemType === 1 || itemType === '1' || itemType === 'cargo' ? 'cargo' : 'item'
}
