(function () {
  'use strict'
  if (window.parent === window) return

  var TAG = 'bitdash-map'
  var READY_POLL_MS = 100
  var READY_MAX_TRIES = 300
  var HASH_POLL_MS = 2000
  var ready = false
  var lastHash = null

  var css = document.createElement('style')
  css.id = 'bitdash-map-bridge-css'
  css.textContent = [
    ':root { --app-header-h: 0px !important; }',
    'body > div > .min-h-screen > header, body > div > .min-h-screen > footer { display: none !important; }',
  ].join('\n')
  document.head.appendChild(css)

  function send(type, data) {
    window.parent.postMessage({ source: TAG, type: type, data: data }, location.origin)
  }
  function currentHash() {
    return location.hash.replace(/^#/, '')
  }
  // The map keeps everything a tile needs (view, layers, tracked players, resources and
  // creatures) in its hash, so that is the state the dashboard saves and restores.
  function reportHash() {
    if (!ready) return
    var hash = currentHash()
    if (hash === lastHash) return
    lastHash = hash
    send('hash', { hash: hash })
  }
  function announce() {
    if (!ready) return
    lastHash = currentHash()
    send('ready', { hash: lastHash })
  }

  ;['pushState', 'replaceState'].forEach(function (name) {
    var original = history[name]
    history[name] = function () {
      var result = original.apply(this, arguments)
      setTimeout(reportHash, 0)
      return result
    }
  })
  window.addEventListener('hashchange', reportHash)
  setInterval(reportHash, HASH_POLL_MS)

  var tries = 0
  var readyTimer = setInterval(function () {
    tries += 1
    if (document.querySelector('[role="application"] canvas') || tries > READY_MAX_TRIES) {
      clearInterval(readyTimer)
      ready = true
      announce()
    }
  }, READY_POLL_MS)

  window.addEventListener('message', function (event) {
    if (event.source !== window.parent || event.origin !== location.origin) return
    var msg = event.data
    if (!msg || msg.target !== TAG) return
    if (msg.cmd === 'hello') announce()
  })
})()
