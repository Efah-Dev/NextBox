// Drag to reorder a vertical list of equally spaced items.
//
// While dragging, nothing in the list moves in the DOM: the lifted item
// follows the pointer and the items it passes slide out of its way, both with
// the CSS `translate` property (which the cards' `transform` entrance
// animation does not override). On drop, onDrop reorders the data and the
// slide-out offsets are dropped in the same frame, without animating back.
import { reactive, type Ref } from 'vue'

interface Options {
  /** the scrolling element that holds the items */
  container: Ref<HTMLElement | undefined>
  /** selector of one item inside the container */
  item: string
  count: () => number
  enabled: () => boolean
  /** presses on these (buttons, inputs) never start a drag */
  ignore?: string
  onDrop: (from: number, to: number) => void
}

export function useDragSort(o: Options) {
  const drag = reactive({ active: false, settling: false, from: -1, to: -1, dy: 0, pitch: 0 })
  let pending: { index: number; y: number } | null = null
  let startY = 0
  let startScroll = 0
  let lastY = 0
  let raf = 0
  let justDropped = false

  function start(e: PointerEvent, index: number) {
    if (e.button !== 0 || !o.enabled()) return
    if (o.ignore && (e.target as HTMLElement).closest(o.ignore)) return
    pending = { index, y: e.clientY }
    lastY = e.clientY
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', end)
    window.addEventListener('pointercancel', end)
  }

  function begin() {
    const el = o.container.value
    if (!el || !pending) return
    const items = el.querySelectorAll<HTMLElement>(o.item)
    if (items.length < 2) return
    drag.pitch = items[1].offsetTop - items[0].offsetTop
    drag.from = drag.to = pending.index
    drag.dy = 0
    drag.active = true
    startY = pending.y
    startScroll = el.scrollTop
    document.body.classList.add('sorting')
    raf = requestAnimationFrame(autoScroll)
  }

  function move(e: PointerEvent) {
    lastY = e.clientY
    if (!drag.active && pending && Math.abs(e.clientY - pending.y) > 4) begin()
    if (drag.active) place()
  }

  function place() {
    const el = o.container.value
    if (!el) return
    drag.dy = lastY - startY + (el.scrollTop - startScroll)
    drag.to = Math.max(0, Math.min(o.count() - 1, Math.round(drag.from + drag.dy / drag.pitch)))
  }

  // scroll the list while the pointer is held near its top or bottom edge
  function autoScroll() {
    const el = o.container.value
    if (!drag.active || !el) return
    const r = el.getBoundingClientRect()
    const edge = 44
    let v = 0
    if (lastY < r.top + edge) v = -Math.ceil((r.top + edge - lastY) / 3)
    else if (lastY > r.bottom - edge) v = Math.ceil((lastY - r.bottom + edge) / 3)
    if (v) {
      el.scrollTop += v
      place()
    }
    raf = requestAnimationFrame(autoScroll)
  }

  function end() {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', end)
    window.removeEventListener('pointercancel', end)
    cancelAnimationFrame(raf)
    pending = null
    if (!drag.active) return
    document.body.classList.remove('sorting')
    const { from, to } = drag
    // the click that follows this pointerup must not select the item
    justDropped = true
    setTimeout(() => (justDropped = false))
    drag.settling = true
    drag.active = false
    if (from !== to) o.onDrop(from, to)
    requestAnimationFrame(() => requestAnimationFrame(() => (drag.settling = false)))
  }

  /** Inline style for the item at index. */
  function style(index: number): Record<string, string> | undefined {
    if (!drag.active) return undefined
    const { from, to, pitch } = drag
    if (index === from) return { translate: `0 ${drag.dy}px` }
    if (from < to && index > from && index <= to) return { translate: `0 ${-pitch}px` }
    if (from > to && index >= to && index < from) return { translate: `0 ${pitch}px` }
    return undefined
  }

  /** Classes for the item at index (see style.css: .sort-lifted, .sort-shift). */
  function classes(index: number) {
    return { 'sort-lifted': drag.active && index === drag.from, 'sort-shift': drag.active && index !== drag.from }
  }

  return { drag, start, style, classes, wasDragged: () => justDropped }
}

/** Moves the item at from to to, in place. */
export function moveItem<T>(list: T[], from: number, to: number) {
  const [item] = list.splice(from, 1)
  list.splice(to, 0, item)
}
