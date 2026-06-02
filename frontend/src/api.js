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

/**
 * Fetch data from a BitJita API path via the backend caching proxy.
 * @param {string} path - e.g. '/status', '/regions/status', '/status/chart'
 * @param {Record<string,string>} params - optional query params
 */
export async function fetchBitjita(path, params = {}) {
  const url = new URL('/api/bitjita' + path, window.location.origin)
  Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v))
  const res = await fetch(url.toString())
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

/**
 * POST to BitJita API via backend proxy (no GET cache).
 * @param {string} path - e.g. '/market/prices/bulk'
 * @param {unknown} jsonBody - serializable body
 */
export async function postBitjita(path, jsonBody) {
  const url = new URL('/api/bitjita' + path, window.location.origin)
  const res = await fetch(url.toString(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(jsonBody ?? {}),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}
