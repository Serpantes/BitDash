(function () {
  'use strict'
  if (window.parent === window) return

  var TAG = 'bitdash-map'
  var PANELS = ['Layers', 'Features', 'Tracking']
  var SEARCH_WATCH_MS = 3500
  var SEARCH_MAX_WATCH_MS = 15000
  var REVEAL_WATCH_MS = 8000
  var map = null
  var realL

  try {
    Object.defineProperty(window, 'L', {
      configurable: true,
      get: function () { return realL },
      set: function (value) {
        realL = value
        try {
          value.Map.addInitHook(function () {
            if (!map) {
              map = this
              onMap()
            }
          })
        } catch (e) { }
      },
    })
  } catch (e) { }

  // Upstream's relay client only reconnects after a connect error: a close without one
  // (server restart, idle timeout, network drop) leaves every tracked player frozen
  // for good. Such closes are reported to it as errors, so its own retry path rebuilds
  // the connection and resubscribes; sockets that stay silent across a sleep or a
  // network change are recycled the same way.
  var RELAY_HOST = /(^|\.)prism\.brico\.app$/
  var RELAY_OPEN_TIMEOUT_MS = 20000
  var RELAY_STALE_MS = 60000
  var relaySockets = []
  var relayState = ''
  var NativeWebSocket = window.WebSocket

  function isRelayUrl(url) {
    try {
      return RELAY_HOST.test(new URL(String(url), location.href).hostname)
    } catch (e) {
      return false
    }
  }
  function setRelayState(state) {
    if (state === relayState) return
    relayState = state
    send('relay', { state: state })
  }
  function watchRelay(socket) {
    var entry = { socket: socket, openedAt: Date.now(), lastMessageAt: 0, errored: false, intended: false }
    var nativeClose = socket.close
    entry.kill = function () { nativeClose.call(socket) }
    socket.close = function () {
      entry.intended = true
      return nativeClose.apply(socket, arguments)
    }
    socket.addEventListener('open', function () {
      entry.lastMessageAt = Date.now()
      setRelayState('live')
    })
    socket.addEventListener('message', function () { entry.lastMessageAt = Date.now() })
    socket.addEventListener('error', function () { entry.errored = true })
    socket.addEventListener('close', function () {
      relaySockets = relaySockets.filter(function (other) { return other !== entry })
      if (entry.intended) {
        if (!relaySockets.length) setRelayState('')
        return
      }
      setRelayState('reconnecting')
      if (entry.errored || typeof socket.onerror !== 'function') return
      entry.errored = true
      try { socket.onerror(new Event('error')) } catch (e) { }
    })
    relaySockets.push(entry)
  }
  function recycleRelay(mode) {
    var now = Date.now()
    relaySockets.slice().forEach(function (entry) {
      var state = entry.socket.readyState
      if (state === NativeWebSocket.CONNECTING) {
        if (now - entry.openedAt > RELAY_OPEN_TIMEOUT_MS) entry.kill()
      } else if (state === NativeWebSocket.OPEN) {
        if (mode === 'force' || (mode === 'stale' && now - entry.lastMessageAt > RELAY_STALE_MS)) entry.kill()
      }
    })
  }
  if (NativeWebSocket) {
    var RelayAwareWebSocket = function (url, protocols) {
      var socket = protocols !== undefined ? new NativeWebSocket(url, protocols) : new NativeWebSocket(url)
      if (isRelayUrl(url)) watchRelay(socket)
      return socket
    }
    RelayAwareWebSocket.prototype = NativeWebSocket.prototype
    ;['CONNECTING', 'OPEN', 'CLOSING', 'CLOSED'].forEach(function (key) { RelayAwareWebSocket[key] = NativeWebSocket[key] })
    window.WebSocket = RelayAwareWebSocket

    var lastTick = Date.now()
    setInterval(function () {
      var now = Date.now()
      // hidden frames get their timers throttled to about once a minute; that is not sleep
      var slept = now - lastTick > 30000 && document.visibilityState === 'visible'
      lastTick = now
      recycleRelay(slept ? 'force' : 'connecting')
    }, 5000)
    window.addEventListener('online', function () { recycleRelay('force') })
    document.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'visible') recycleRelay('stale')
    })
  }

  var css = document.createElement('style')
  css.id = 'bitdash-map-bridge-css'
  css.textContent = [
    '.relative.h-viewport > *:not(.z-map) { display: none !important; }',
    '.leaflet-container { background: #15131d !important; font-family: Nunito, system-ui, sans-serif; }',
    '.leaflet-popup-content-wrapper { background: #2a2737 !important; color: #f2ebd9 !important;',
    '  border: 1px solid rgba(233, 223, 196, 0.45); border-radius: 4px !important; box-shadow: 0 10px 24px rgba(0, 0, 0, 0.5) !important; }',
    '.leaflet-popup-tip { background: #2a2737 !important; }',
    '.leaflet-popup-close-button { color: #d9d1bd !important; }',
    '.leaflet-tooltip { background: rgba(24, 22, 34, 0.95) !important; color: #f2ebd9 !important;',
    '  border: 1px solid rgba(233, 223, 196, 0.45) !important; border-radius: 3px !important; box-shadow: none !important; }',
    '.leaflet-tooltip.bitdash-hover-tip { padding: 2px 7px; font-size: 12px; font-weight: 700; white-space: nowrap; pointer-events: none; }',
    '.leaflet-tooltip.bitdash-hover-tip::before { display: none; }',
    '.leaflet-container.bitdash-hover-player { cursor: pointer !important; }',
  ].join('\n')
  document.head.appendChild(css)

  var valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set

  function send(type, data) {
    window.parent.postMessage({ source: TAG, type: type, data: data }, location.origin)
  }
  function wait(ms) {
    return new Promise(function (resolve) { setTimeout(resolve, ms) })
  }
  function text(el) {
    return el ? el.textContent.replace(/\s+/g, ' ').trim() : ''
  }
  function toGame(latlng) {
    return { n: Math.round(latlng.lat / 3), e: Math.round(latlng.lng / 3) }
  }
  function throttle(fn, ms) {
    var last = 0
    var timer = null
    return function (arg) {
      var now = Date.now()
      clearTimeout(timer)
      if (now - last >= ms) {
        last = now
        fn(arg)
      } else {
        timer = setTimeout(function () { last = Date.now(); fn(arg) }, ms - (now - last))
      }
    }
  }

  function isReady() {
    return Boolean(map && document.querySelector('.z-search input'))
  }
  function viewState() {
    var c = toGame(map.getCenter())
    return { n: c.n, e: c.e, zoom: Math.round(map.getZoom() * 10) / 10 }
  }
  function announce() {
    if (isReady()) send('ready', { view: viewState(), query: trackingSearch(), relay: relayState })
  }
  var HOVER_PAD_PX = 3
  var hoverTip = null
  var hoverMarker = null
  var hoverFrame = 0
  function popupPlayerName(layer) {
    var popup = layer.getPopup && layer.getPopup()
    if (!popup) return ''
    var content = popup.getContent()
    if (typeof content === 'function') {
      try { content = content(layer) } catch (e) { return '' }
    }
    if (content && content.nodeType === 1) content = content.outerHTML
    if (typeof content !== 'string' || content.indexOf('bcm-popup-type') < 0) return ''
    var doc = new DOMParser().parseFromString(content, 'text/html')
    if (text(doc.querySelector('.bcm-popup-type')) !== 'Player') return ''
    return text(doc.querySelector('.bcm-popup-name'))
  }
  function showHover(latlng, name, offsetY) {
    if (!hoverTip) hoverTip = realL.tooltip({ direction: 'top', className: 'bitdash-hover-tip', opacity: 1, interactive: false })
    var label = document.createElement('span')
    label.textContent = name
    hoverTip.options.offset = realL.point(0, -offsetY)
    hoverTip.setContent(label)
    hoverTip.setLatLng(latlng)
    if (!map.hasLayer(hoverTip)) map.openTooltip(hoverTip)
  }
  function hideHover() {
    hoverMarker = null
    map.getContainer().classList.remove('bitdash-hover-player')
    if (hoverTip && map.hasLayer(hoverTip)) map.closeTooltip(hoverTip)
  }
  function onMarkerOver(e) {
    var layer = e.target
    if (layer.isPopupOpen && layer.isPopupOpen()) return
    var name = popupPlayerName(layer)
    if (!name) return
    var el = layer.getElement && layer.getElement()
    var box = map.getContainer().getBoundingClientRect()
    var latlng = layer.getLatLng()
    if (el) {
      var rect = el.getBoundingClientRect()
      latlng = map.containerPointToLatLng([rect.left + rect.width / 2 - box.left, rect.top - box.top])
    }
    hoverMarker = layer
    showHover(latlng, name, 2)
  }
  function onMarkerOut(e) {
    if (hoverMarker === e.target) hideHover()
  }
  function canvasPlayerAt(point) {
    var best = null
    map.eachLayer(function (layer) {
      var points = layer._drawnScreenPoints
      var ids = layer._drawnEntityIds
      var names = layer._drawnNames
      var coords = layer._drawnGameCoords
      if (!points || !points.length || !ids || !names || !coords) return
      var radius = (layer._radius || 4) + HOVER_PAD_PX
      var limit = radius * radius
      for (var i = 0; i < points.length; i += 2) {
        var dx = points[i] - point.x
        var dy = points[i + 1] - point.y
        var dist = dx * dx + dy * dy
        if (dist < limit && (!best || dist < best.dist) && ids[i / 2]) {
          best = { dist: dist, name: names[i / 2] || 'Player', latlng: [coords[i], coords[i + 1]], radius: layer._radius || 4 }
        }
      }
    })
    return best
  }
  function onHoverMove(e) {
    if (hoverMarker) return
    cancelAnimationFrame(hoverFrame)
    hoverFrame = requestAnimationFrame(function () {
      if (hoverMarker) return
      var hit = canvasPlayerAt(e.containerPoint)
      if (!hit) {
        hideHover()
        return
      }
      map.getContainer().classList.add('bitdash-hover-player')
      showHover(hit.latlng, hit.name, hit.radius + 2)
    })
  }
  function watchHover() {
    map.on('layeradd', function (e) {
      var layer = e.layer
      if (!(layer instanceof realL.Marker) || layer._bitdashHover) return
      layer._bitdashHover = true
      layer.on('mouseover', onMarkerOver)
      layer.on('mouseout', onMarkerOut)
    })
    map.on('layerremove', function (e) { if (e.layer === hoverMarker) hideHover() })
    map.on('mousemove', onHoverMove)
    map.on('mouseout movestart zoomstart popupopen', function () {
      cancelAnimationFrame(hoverFrame)
      hideHover()
    })
  }

  function onMap() {
    map.on('moveend', throttle(function () { send('view', viewState()) }, 400))
    map.on('mousemove', throttle(function (e) { send('cursor', toGame(e.latlng)) }, 60))
    map.on('mouseout', function () { send('cursor', null) })
    watchHover()
    var tries = 0
    var timer = setInterval(function () {
      tries += 1
      if (isReady() || tries > 100) {
        clearInterval(timer)
        announce()
        setInterval(sendTracked, TRACKED_POLL_MS)
      }
    }, 100)
  }

  // Upstream keeps "Follow Player" out of the URL; it only reads followPlayer at load and
  // then follows the first playerId, so the saved query carries both.
  var followId = initialFollowId()
  function playerIds(params) {
    return (params.get('playerId') || '').split(',').map(function (id) { return id.trim() }).filter(Boolean)
  }
  function initialFollowId() {
    var params = new URLSearchParams(location.search)
    var follow = String(params.get('followPlayer') || '').toLowerCase()
    return follow === 'true' || follow === '1' ? playerIds(params)[0] || null : null
  }
  function trackingSearch() {
    var params = new URLSearchParams(location.search)
    var ids = playerIds(params)
    if (followId && ids.indexOf(followId) >= 0) {
      params.set('playerId', [followId].concat(ids.filter(function (id) { return id !== followId })).join(','))
      params.set('followPlayer', '1')
    } else {
      params.delete('followPlayer')
    }
    var query = params.toString().replace(/%2C/g, ',')
    return query ? '?' + query : ''
  }
  function sendQuery() {
    send('query', { query: trackingSearch() })
  }
  document.addEventListener('click', function (e) {
    var button = e.target && e.target.closest && e.target.closest('[data-action="follow-player"]')
    if (!button || !button.dataset.entityId) return
    followId = followId === button.dataset.entityId ? null : button.dataset.entityId
    setTimeout(sendQuery, 0)
  }, true)

  var TRACKED_REREAD_MS = [300, 1500, 4000, 8000, 15000]
  var TRACKED_POLL_MS = 15000
  var trackedTimers = []
  ;['pushState', 'replaceState'].forEach(function (name) {
    var original = history[name]
    history[name] = function () {
      var result = original.apply(this, arguments)
      sendQuery()
      trackedTimers.forEach(clearTimeout)
      trackedTimers = TRACKED_REREAD_MS.map(function (ms) { return setTimeout(sendTracked, ms) })
      return result
    }
  })

  function panelButton(name) {
    return document.querySelector('.sm\\:flex button[aria-label="' + name + '"]')
      || document.querySelector('button[aria-label="' + name + '"]')
  }
  function findPanel(name) {
    var headings = document.querySelectorAll('h2')
    for (var i = 0; i < headings.length; i += 1) {
      if (text(headings[i]).toLowerCase() === name.toLowerCase()) return headings[i].parentElement.parentElement
    }
    return null
  }
  var panelQueue = Promise.resolve()
  function enqueue(task) {
    panelQueue = panelQueue.then(task, task)
    return panelQueue
  }
  function withPanel(name, fn) {
    var panel = findPanel(name)
    var opened = false
    if (!panel) {
      var button = panelButton(name)
      if (!button) return Promise.resolve(null)
      button.click()
      opened = true
    }
    return waitForPanel(name).then(function (found) {
      return Promise.resolve(found ? fn(found) : null).then(function (result) {
        if (opened && findPanel(name)) panelButton(name).click()
        return result
      })
    })
  }
  function waitForPanel(name) {
    var started = Date.now()
    return new Promise(function (resolve) {
      ;(function poll() {
        var panel = findPanel(name)
        if (panel || Date.now() - started > 800) resolve(panel)
        else setTimeout(poll, 20)
      })()
    })
  }
  function sectionTitle(input, panel) {
    var node = input.parentElement
    while (node && node !== panel) {
      var heading = node.querySelector(':scope > h3')
      if (heading) return text(heading)
      node = node.parentElement
    }
    return ''
  }
  function collectControls(panel, panelName) {
    var controls = []
    panel.querySelectorAll('input[type=checkbox], input[type=radio]').forEach(function (input) {
      var row = input.closest('label, summary') || input.parentElement
      if (row.querySelector('button[aria-label="Remove tracking"]')) return
      var isGroup = row.tagName === 'SUMMARY'
      var details = input.closest('details')
      var label = row.tagName === 'LABEL' || isGroup
        ? text(row)
        : text(row.querySelector('span.flex-1') || row)
      var group = isGroup ? '' : (details ? text(details.querySelector('summary')) : sectionTitle(input, panel))
      controls.push({
        input: input,
        item: {
          id: panelName + '|' + group + '|' + label,
          panel: panelName,
          group: group,
          label: label,
          type: input.type,
          checked: input.checked,
          indeterminate: Boolean(input.indeterminate),
          isGroup: isGroup,
        },
      })
    })
    return controls
  }
  function parseControls(panel, panelName) {
    return collectControls(panel, panelName).map(function (control) { return control.item })
  }
  function trackedRowLabel(row) {
    return text(row.querySelector('span.flex-1') || row).replace(/^(Tracking|Player):\s*/i, '')
  }
  function parseTracked(panel) {
    var items = []
    panel.querySelectorAll('button[aria-label="Remove tracking"]').forEach(function (remove) {
      var row = remove.parentElement
      var checkbox = row.querySelector('input[type=checkbox]')
      var swatch = row.querySelector('button[aria-label="Change color"]')
      items.push({
        label: trackedRowLabel(row),
        kind: /^Player:/i.test(text(row.querySelector('span.flex-1') || row)) ? 'player' : 'item',
        visible: checkbox ? checkbox.checked : true,
        color: swatch ? swatch.style.backgroundColor : '',
      })
    })
    return items
  }
  function findInput(panel, panelName, id) {
    var match = collectControls(panel, panelName).filter(function (control) { return control.item.id === id })[0]
    return match ? match.input : null
  }
  function readControls() {
    var all = []
    return PANELS.reduce(function (chain, name) {
      return chain.then(function () {
        return withPanel(name, function (panel) { all = all.concat(parseControls(panel, name)) })
      })
    }, Promise.resolve()).then(function () { return all })
  }
  function sendControls() {
    return enqueue(function () {
      return readControls().then(function (items) { send('controls', { items: items }) })
    })
  }
  var lastTracked = null
  function reportTracked(items) {
    var signature = JSON.stringify(items)
    if (signature === lastTracked) return
    lastTracked = signature
    send('tracked', { items: items })
  }
  function sendTracked() {
    return enqueue(function () {
      return withPanel('Tracking', function (panel) { reportTracked(parseTracked(panel)) })
    })
  }
  function applyControls(states) {
    return enqueue(function () {
      return PANELS.reduce(function (chain, name) {
      return chain.then(function () {
        return withPanel(name, function (panel) {
          Object.keys(states).forEach(function (id) {
            if (id.split('|')[0] !== name) return
            var input = findInput(panel, name, id)
            if (input && input.checked !== Boolean(states[id]) && (input.type === 'checkbox' || states[id])) input.click()
          })
          return wait(30)
        })
      })
      }, Promise.resolve())
    }).then(sendControls)
  }
  function withTrackedRow(label, fn) {
    return enqueue(function () {
      return withPanel('Tracking', function (panel) {
      var rows = panel.querySelectorAll('button[aria-label="Remove tracking"]')
      for (var i = 0; i < rows.length; i += 1) {
        var row = rows[i].parentElement
        if (trackedRowLabel(row) === label) {
          fn(row)
          break
        }
      }
      return wait(40).then(function () { reportTracked(parseTracked(panel)) })
      })
    })
  }

  var searchRun = 0
  function searchInput() {
    return document.querySelector('.z-search input')
  }
  function setSearchValue(value) {
    var input = searchInput()
    if (!input) return null
    input.dispatchEvent(new FocusEvent('focus'))
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))
    valueSetter.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    return input
  }
  function resultsContainer() {
    var box = document.querySelector('.z-search')
    return box ? box.querySelector(':scope > div:not(.relative)') : null
  }
  function parseResultItem(main) {
    var meta = text(main.querySelector('.ml-auto'))
    var label = text(main)
    if (meta && label.slice(-meta.length) === meta) label = label.slice(0, -meta.length).trim()
    var swatch = main.querySelector('span[style*="background"]')
    var item = { label: label, meta: meta, color: swatch ? swatch.style.backgroundColor : '' }
    var coords = label.match(/^(.*?)\s+N\s+(-?\d+)\s+E\s+(-?\d+)$/)
    if (coords) {
      item.label = coords[1]
      item.n = Number(coords[2])
      item.e = Number(coords[3])
    }
    return item
  }
  function readResultRows() {
    var container = resultsContainer()
    var rows = []
    var title = ''
    if (!container) return rows
    Array.prototype.forEach.call(container.children, function (el) {
      var main = el.querySelector('button')
      if (!main) {
        if (text(el)) title = text(el)
        return
      }
      rows.push({ group: title, button: main, item: parseResultItem(main) })
    })
    return rows
  }
  function parseResults() {
    var groups = []
    var current = null
    readResultRows().forEach(function (row) {
      if (!current || current.title !== row.group) {
        current = { title: row.group, items: [] }
        groups.push(current)
      }
      current.items.push(row.item)
    })
    return groups
  }
  function groupKey(title) {
    return String(title || '').split(' ')[0]
  }
  function resultSignature(item) {
    return [item.label, item.meta, item.n, item.e].map(function (v) { return v == null ? '' : String(v) }).join('|')
  }
  function findResultButton(target) {
    var wanted = resultSignature(target)
    var key = groupKey(target.group)
    var matches = []
    readResultRows()
      .filter(function (row) { return !target.group || groupKey(row.group) === key })
      .forEach(function (row, position) {
        if (resultSignature(row.item) === wanted) matches.push({ position: position, button: row.button })
      })
    var exact = matches.filter(function (match) { return match.position === target.position })[0]
    return (exact || matches[0] || {}).button || null
  }
  function search(q, seq) {
    var run = ++searchRun
    if (!setSearchValue(q)) {
      send('results', { seq: seq, q: q, groups: [], done: true, error: 'unavailable' })
      return
    }
    var started = Date.now()
    var last = null
    ;(function poll() {
      if (run !== searchRun) return
      var groups = parseResults()
      var signature = JSON.stringify(groups)
      var elapsed = Date.now() - started
      var pending = !groups.length || groups.some(function (group) { return /(\.\.\.|\u2026)$/.test(group.title) })
      var done = elapsed > SEARCH_MAX_WATCH_MS || (elapsed > SEARCH_WATCH_MS && !pending)
      if (signature !== last || done) {
        last = signature
        send('results', { seq: seq, q: q, groups: groups, done: done })
      }
      if (!done) setTimeout(poll, 150)
    })()
  }
  function revealAfterPick() {
    if (!map) return
    var moved = false
    var added = []
    function onMove() { moved = true }
    function onAdd(e) { if (e.layer && typeof e.layer.getLatLng === 'function') added.push(e.layer) }
    map.on('movestart', onMove)
    map.on('layeradd', onAdd)
    var started = Date.now()
    ;(function check() {
      if (moved || (added.length > 0 && added.length <= 3)) {
        map.off('movestart', onMove)
        map.off('layeradd', onAdd)
        if (!moved && added.length) map.flyTo(added[0].getLatLng(), Math.max(map.getZoom(), 0))
        return
      }
      if (Date.now() - started > REVEAL_WATCH_MS || added.length > 3) {
        map.off('movestart', onMove)
        map.off('layeradd', onAdd)
        return
      }
      setTimeout(check, 250)
    })()
  }
  function pick(target) {
    var button = findResultButton(target)
    if (!button) {
      send('picked', { ok: false })
      return
    }
    revealAfterPick()
    button.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, view: window }))
    searchRun += 1
    setTimeout(function () {
      setSearchValue('')
      sendTracked()
      send('picked', { ok: true })
    }, 120)
  }

  var commands = {
    hello: announce,
    zoomIn: function () { if (map) map.zoomIn() },
    zoomOut: function () { if (map) map.zoomOut() },
    reset: function () {
      var button = document.querySelector('button[aria-label="Reset map view"]')
      if (button) button.click()
    },
    flyTo: function (a) { if (map) map.flyTo([a.n * 3, a.e * 3], a.zoom != null ? a.zoom : map.getZoom()) },
    search: function (a) { search(String(a.q || ''), a.seq) },
    clearSearch: function () { searchRun += 1; setSearchValue('') },
    pick: function (a) {
      pick({ group: a.group || '', label: String(a.label || ''), meta: a.meta, n: a.n, e: a.e, position: a.position })
    },
    controls: sendControls,
    toggle: function (a) {
      var name = String(a.id || '').split('|')[0]
      enqueue(function () {
        return withPanel(name, function (panel) {
          var input = findInput(panel, name, a.id)
          if (input) input.click()
          return wait(30)
        })
      }).then(sendControls)
    },
    setControls: function (a) { applyControls(a.states || {}) },
    tracked: function () {
      lastTracked = null
      sendTracked()
    },
    untrack: function (a) {
      withTrackedRow(a.label, function (row) { row.querySelector('button[aria-label="Remove tracking"]').click() })
    },
    showTracked: function (a) {
      withTrackedRow(a.label, function (row) {
        var checkbox = row.querySelector('input[type=checkbox]')
        if (checkbox && checkbox.checked !== Boolean(a.visible)) checkbox.click()
      })
    },
  }

  window.addEventListener('message', function (event) {
    if (event.source !== window.parent || event.origin !== location.origin) return
    var msg = event.data
    if (!msg || msg.target !== TAG || !commands[msg.cmd]) return
    try {
      commands[msg.cmd](msg.args || {})
    } catch (e) {
      send('error', { cmd: msg.cmd, message: String(e && e.message || e) })
    }
  })
})()
