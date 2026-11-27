// Short highlights that fade away on their own.

/** Highlights el for ms (the last 20% fade out). Calling again restarts it. */
export function flash(el: Element | null | undefined, ms = 3000) {
  if (!(el instanceof HTMLElement)) return
  el.style.animation = 'none'
  void el.offsetWidth // restart
  el.style.animation = `nb-flash ${ms}ms ease-out`
  const done = () => {
    el.style.animation = ''
    el.removeEventListener('animationend', done)
  }
  el.addEventListener('animationend', done)
}
