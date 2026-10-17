import { onUnmounted, reactive, watch } from 'vue'
import { evaluateTileDrop } from '../dashboardLayout'

const DRAG_THRESHOLD_PX = 5
const EDGE_ZONE_PX = 72
const MAX_SCROLL_PX = 22
const OUTSIDE_MARGIN_PX = 48
const SLIDE_MS = 240

function prefersReducedMotion() {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

export function useTileDrag({ gridRef, rootRef, layout, onDrop, headerSelector = '.app-header' }) {
  const drag = reactive({ id: null, target: null })
  let session = null
  let raf = 0

  function onTilePointerDown(event, id) {
    if (event.button !== 0 || !event.isPrimary || session) return
    if (event.target.closest('button, a, input, select, textarea')) return
    const card = event.currentTarget.closest('[data-tile-id]')
    if (!card) return
    event.preventDefault()
    const rect = card.getBoundingClientRect()
    session = {
      id,
      card,
      rect,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      lastX: event.clientX,
      lastY: event.clientY,
      grabX: event.clientX - rect.left,
      grabY: event.clientY - rect.top,
      active: false,
      ghost: null,
      metrics: null,
    }
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
    window.addEventListener('pointercancel', cancelDrag)
    window.addEventListener('keydown', onKeyDown)
  }

  function begin() {
    document.activeElement?.blur?.()
    session.active = true
    session.metrics = gridMetrics()
    session.ghost = makeGhost(session.card, session.rect)
    drag.id = session.id
    document.body.classList.add('tile-dragging')
    raf = requestAnimationFrame(autoScroll)
  }

  function onPointerMove(event) {
    if (!session || event.pointerId !== session.pointerId) return
    session.lastX = event.clientX
    session.lastY = event.clientY
    if (!session.active) {
      if (Math.hypot(event.clientX - session.startX, event.clientY - session.startY) < DRAG_THRESHOLD_PX) return
      begin()
    }
    moveGhost()
    updateTarget()
  }

  function onPointerUp(event) {
    if (!session || event.pointerId !== session.pointerId) return
    end(drag.target?.valid ? drag.target : null)
  }

  function onKeyDown(event) {
    if (event.key === 'Escape') cancelDrag()
  }

  function cancelDrag() {
    if (session) end(null)
  }

  function end(target) {
    const { id, active } = session
    const from = active ? ghostPosition() : null
    cancelAnimationFrame(raf)
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', onPointerUp)
    window.removeEventListener('pointercancel', cancelDrag)
    window.removeEventListener('keydown', onKeyDown)
    session.ghost?.remove()
    session = null
    drag.id = null
    drag.target = null
    document.body.classList.remove('tile-dragging')
    if (active) onDrop(id, target, from)
  }

  function gridMetrics() {
    const style = getComputedStyle(gridRef.value)
    const col = parseFloat(style.gridTemplateColumns) || 380
    const row = parseFloat(style.gridAutoRows) || 446
    return {
      colPitch: col + (parseFloat(style.columnGap) || 0),
      rowPitch: row + (parseFloat(style.rowGap) || 0),
    }
  }

  function updateTarget() {
    const grid = gridRef.value
    if (!session?.active || !grid) return
    const box = grid.getBoundingClientRect()
    const { lastX, lastY, grabX, grabY, metrics } = session
    const outside = lastX < box.left - OUTSIDE_MARGIN_PX || lastX > box.right + OUTSIDE_MARGIN_PX
      || lastY < box.top - OUTSIDE_MARGIN_PX || lastY > box.bottom + OUTSIDE_MARGIN_PX
    let next = null
    if (!outside) {
      const current = layout.value
      const span = current.positions[session.id]?.span ?? 1
      const x = Math.min(Math.max(Math.round((lastX - grabX - box.left) / metrics.colPitch), 0), current.cols - span)
      const y = Math.min(Math.max(Math.round((lastY - grabY - box.top) / metrics.rowPitch), 0), current.rows)
      next = evaluateTileDrop(current, session.id, x, y)
    }
    const prev = drag.target
    if (!prev || !next || prev.x !== next.x || prev.y !== next.y || prev.valid !== next.valid || prev.swap?.id !== next.swap?.id) {
      drag.target = next
    }
  }

  function autoScroll() {
    if (!session?.active) return
    const top = document.querySelector(headerSelector)?.getBoundingClientRect().bottom ?? 0
    const y = session.lastY
    let speed = 0
    if (y < top + EDGE_ZONE_PX) speed = -Math.min(1, (top + EDGE_ZONE_PX - y) / EDGE_ZONE_PX)
    else if (y > window.innerHeight - EDGE_ZONE_PX) speed = Math.min(1, (y - (window.innerHeight - EDGE_ZONE_PX)) / EDGE_ZONE_PX)
    if (speed) {
      window.scrollBy(0, Math.round(speed * MAX_SCROLL_PX))
      updateTarget()
    }
    raf = requestAnimationFrame(autoScroll)
  }

  function makeGhost(card, rect) {
    const ghost = card.cloneNode(true)
    ghost.querySelectorAll('iframe').forEach((frame) => {
      const stub = document.createElement('div')
      stub.style.cssText = 'flex:1;border-radius:3px;background:#0c0b14'
      frame.replaceWith(stub)
    })
    ghost.removeAttribute('data-tile-id')
    ghost.classList.add('tile-ghost')
    Object.assign(ghost.style, {
      position: 'fixed',
      left: `${rect.left}px`,
      top: `${rect.top}px`,
      width: `${rect.width}px`,
      height: `${rect.height}px`,
      margin: '0',
      zIndex: '60',
      pointerEvents: 'none',
    })
    ;(rootRef.value ?? document.body).appendChild(ghost)
    return ghost
  }

  function moveGhost() {
    const dx = session.lastX - session.startX
    const dy = session.lastY - session.startY
    session.ghost.style.transform = `translate(${dx}px, ${dy}px) rotate(0.8deg) scale(1.015)`
  }

  function ghostPosition() {
    return {
      left: session.rect.left + session.lastX - session.startX,
      top: session.rect.top + session.lastY - session.startY,
    }
  }

  onUnmounted(cancelDrag)

  return { drag, onTilePointerDown, cancelDrag }
}

export function useTileFlip(gridRef, signature) {
  const before = new Map()
  let landing = null

  function tiles() {
    return gridRef.value ? [...gridRef.value.querySelectorAll('[data-tile-id]')] : []
  }
  function slide(el, from) {
    const to = el.getBoundingClientRect()
    const dx = from.left - to.left
    const dy = from.top - to.top
    if ((Math.abs(dx) < 1 && Math.abs(dy) < 1) || prefersReducedMotion()) return
    el.animate(
      [{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'translate(0, 0)' }],
      { duration: SLIDE_MS, easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)' },
    )
  }

  watch(signature, () => {
    before.clear()
    tiles().forEach((el) => before.set(el.dataset.tileId, el.getBoundingClientRect()))
  }, { flush: 'pre' })
  watch(signature, () => {
    tiles().forEach((el) => {
      const id = el.dataset.tileId
      const from = landing?.id === id ? landing : before.get(id)
      if (from) slide(el, from)
    })
    landing = null
    before.clear()
  }, { flush: 'post' })

  function landFrom(id, from) {
    landing = from ? { id, ...from } : null
  }
  function slideBack(id, from) {
    const el = tiles().find((tile) => tile.dataset.tileId === id)
    if (el && from) slide(el, from)
  }

  return { landFrom, slideBack }
}
