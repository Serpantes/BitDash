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
