export const MAP_EMBED_PATH = '/map-embed/map'

function idList(value) {
  return String(value ?? '').split(',').map((id) => id.trim()).filter((id) => /^\d+$/.test(id))
}

// Tiles saved before the switch to bitjita.com/map kept bitcraftmap's query and view;
// carry the tracked players, creatures and resources and the centre over to its hash.
export function legacyMapHash(state, legacyUrl) {
  let search = state?.query ?? ''
  if (!search && legacyUrl) {
    try {
      search = new URL(legacyUrl, 'http://x').search
    } catch {
      search = ''
    }
  }
  const params = new URLSearchParams(search)
  const parts = []
  const view = state?.view
  if (view && Number.isFinite(view.n) && Number.isFinite(view.e)) parts.push(`0/${Math.round(view.n)}/${Math.round(view.e)}`)
  const tracked = [
    ...idList(params.get('resourceId')).map((id) => `res:${id}`),
    ...idList(params.get('enemyId')).map((id) => `mob:${id}`),
  ]
  if (tracked.length) parts.push(`t=${tracked.join(',')}`)
  const pids = idList(params.get('playerId'))
  if (pids.length) {
    parts.push(`pid=${pids.join(',')}`)
    if (['true', '1'].includes(String(params.get('followPlayer') ?? '').toLowerCase())) parts.push('follow=1')
  }
  return parts.join('&')
}

export function mapHash(state, legacyUrl) {
  if (typeof state?.hash === 'string') return state.hash.replace(/^#/, '')
  return legacyMapHash(state, legacyUrl)
}

export function mapEmbedSrc(state, legacyUrl, reload = null) {
  const hash = mapHash(state, legacyUrl)
  return `${MAP_EMBED_PATH}${reload ? `?reload=${reload}` : ''}${hash ? `#${hash}` : ''}`
}
