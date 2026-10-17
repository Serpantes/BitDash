import { t } from './i18n'

export const MAP_EMBED_PATH = '/map-embed/'
const BRIDGE_VERSION = '1'
const TRACKING_PARAMS = ['playerId', 'followPlayer', 'resourceId', 'enemyId', 'regionId']

export function mapLabel(label) {
  const labels = t('mapLabels')
  if (labels[label]) return labels[label]
  const tier = String(label ?? '').match(/^(.+) T(\d+)$/)
  if (tier) return `T${tier[2]}`
  const head = Object.keys(labels).find((key) => String(label ?? '').startsWith(`${key} `))
  if (head) return `${labels[head]}${String(label).slice(head.length)}`
  return label
}

export function trackingQuery(search) {
  const params = new URLSearchParams(search ?? '')
  const kept = new URLSearchParams()
  TRACKING_PARAMS.forEach((key) => {
    const value = params.get(key)
    if (value) kept.set(key, value)
  })
  const text = kept.toString()
  return text ? `?${text}` : ''
}

export function mapEmbedSrc(state, legacyUrl) {
  const params = new URLSearchParams({ bridge: BRIDGE_VERSION })
  let source = state?.query ?? null
  if (source == null && legacyUrl) {
    try {
      source = new URL(legacyUrl, 'http://x').search
    } catch {
      source = ''
    }
  }
  new URLSearchParams(trackingQuery(source)).forEach((value, key) => params.set(key, value))
  const view = state?.view
  if (view && Number.isFinite(view.n) && Number.isFinite(view.e) && Number.isFinite(view.zoom)) {
    params.set('center', `${view.n},${view.e}`)
    params.set('zoom', String(view.zoom))
  }
  return `${MAP_EMBED_PATH}?${params.toString().replaceAll('%2C', ',')}`
}

export function controlStates(items) {
  const states = {}
  ;(items ?? []).forEach((item) => {
    if (item.isGroup) return
    if (item.type === 'radio' && !item.checked) return
    states[item.id] = item.checked
  })
  return states
}

export function controlsDiffer(items, saved) {
  if (!saved) return false
  const current = controlStates(items)
  return Object.entries(saved).some(([id, checked]) => (current[id] ?? false) !== checked)
}

export function shapeControls(items) {
  const list = items ?? []
  const byPanel = (panel) => list.filter((item) => item.panel === panel)
  const layers = byPanel('Layers')
  const baseMaps = layers.filter((item) => item.type === 'radio')
  const overlays = layers.filter((item) => item.type === 'checkbox')
  const players = byPanel('Tracking')
  const features = byPanel('Features')
  const groups = features.filter((item) => item.isGroup).map((group) => {
    const children = features.filter((item) => !item.isGroup && item.group === group.label)
    const isTier = (item) => /^.+ T\d+$/.test(item.label)
    return { item: group, named: children.filter((c) => !isTier(c)), tiers: children.filter(isTier) }
  })
  return { baseMaps, overlays, players, groups }
}

export function trackedLabel(label) {
  return String(label ?? '').replace(/,\s*Tier\s+(-?\d+)$/i, ' · T$1')
}
