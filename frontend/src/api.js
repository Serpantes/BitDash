export async function ping(path, timeout = 5000) {
  const url = path.startsWith('http') ? path : window.location.origin + path
  const start = Date.now()
  try {
    const controller = new AbortController()
    const id = setTimeout(() => controller.abort(), timeout)
    const res = await fetch(url, { signal: controller.signal })
    clearTimeout(id)
    const time = Date.now() - start
    return { url: path, ok: res.ok, status: res.status, time }
  } catch (e) {
    return { url: path, ok: false, status: null, time: null, error: e.name === 'AbortError' ? 'timeout' : e.message }
  }
}

const REQUEST_TIMEOUT_MS = 45000

// A request that never settles would keep its block "refreshing" forever and stall every
// later poll that waits on it, so each one gives up after a bound.
async function fetchWithTimeout(url, init = {}, timeout = REQUEST_TIMEOUT_MS) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeout)
  try {
    return await fetch(url, { ...init, signal: controller.signal })
  } catch (e) {
    if (e.name === 'AbortError') throw new Error('timeout')
    throw e
  } finally {
    clearTimeout(timer)
  }
}

export async function fetchBitjita(path, params = {}, { fresh = false } = {}) {
  const url = new URL('/api/bitjita' + path, window.location.origin)
  Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v))
  const res = await fetchWithTimeout(url.toString(), fresh ? { headers: { 'Cache-Control': 'no-cache' } } : undefined)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function pingBitjita(path = '/status', timeout = 10000) {
  return ping(`/api/bitjita${path}`, timeout)
}

export async function postBitjita(path, jsonBody) {
  const url = new URL('/api/bitjita' + path, window.location.origin)
  const res = await fetchWithTimeout(url.toString(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(jsonBody ?? {}),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function fetchMarketCatalog() {
  const res = await fetch('/api/market-catalog')
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function fetchSmithingOre(playerEntityId) {
  const res = await fetchWithTimeout(`/api/smithing-ore/${playerEntityId}`)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function fetchXpHistory(playerEntityId, since = 0) {
  const url = new URL(`/api/xp-history/${playerEntityId}`, window.location.origin)
  if (since) url.searchParams.set('since', String(since))
  const res = await fetchWithTimeout(url.toString(), {}, 20000)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}
