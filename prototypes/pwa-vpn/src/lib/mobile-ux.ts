/**
 * MOB-UX-001: Mobile UX — touch optimizations
 * - Long-press for context menu
 * - Swipe-to-delete
 * - Haptic feedback
 * - Safe area insets
 */

export function isMobile(): boolean {
  if (typeof window === 'undefined') return false
  return /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent) || window.innerWidth < 768
}

export function isTouchDevice(): boolean {
  return typeof window !== 'undefined' && ('ontouchstart' in window || navigator.maxTouchPoints > 0)
}

/** Long-press handler (500ms) */
export function onLongPress(el: HTMLElement, callback: () => void): () => void {
  let timer: ReturnType<typeof setTimeout> | null = null
  let startX = 0, startY = 0

  const start = (e: TouchEvent | MouseEvent) => {
    const point = 'touches' in e ? e.touches[0] : e
    startX = point.clientX
    startY = point.clientY
    timer = setTimeout(() => {
      vibrate(50)
      callback()
    }, 500)
  }

  const move = (e: TouchEvent | MouseEvent) => {
    if (!timer) return
    const point = 'touches' in e ? e.touches[0] : e
    if (Math.abs(point.clientX - startX) > 10 || Math.abs(point.clientY - startY) > 10) {
      clearTimeout(timer)
      timer = null
    }
  }

  const end = () => {
    if (timer) clearTimeout(timer)
    timer = null
  }

  el.addEventListener('touchstart', start, { passive: true })
  el.addEventListener('touchmove', move, { passive: true })
  el.addEventListener('touchend', end, { passive: true })
  el.addEventListener('mousedown', start)
  el.addEventListener('mousemove', move)
  el.addEventListener('mouseup', end)
  el.addEventListener('mouseleave', end)

  return () => {
    el.removeEventListener('touchstart', start)
    el.removeEventListener('touchmove', move)
    el.removeEventListener('touchend', end)
    el.removeEventListener('mousedown', start)
    el.removeEventListener('mousemove', move)
    el.removeEventListener('mouseup', end)
    el.removeEventListener('mouseleave', end)
  }
}

/** Swipe handler — returns cleanup */
export function onSwipe(el: HTMLElement, onLeft: () => void, onRight: () => void): () => void {
  let startX = 0, startY = 0, startTime = 0

  const start = (e: TouchEvent) => {
    startX = e.touches[0].clientX
    startY = e.touches[0].clientY
    startTime = Date.now()
  }

  const end = (e: TouchEvent) => {
    const dx = e.changedTouches[0].clientX - startX
    const dy = e.changedTouches[0].clientY - startY
    const dt = Date.now() - startTime
    if (dt > 500) return
    if (Math.abs(dx) < 50) return
    if (Math.abs(dx) < Math.abs(dy)) return
    if (dx > 0) onRight()
    else onLeft()
  }

  el.addEventListener('touchstart', start, { passive: true })
  el.addEventListener('touchend', end, { passive: true })

  return () => {
    el.removeEventListener('touchstart', start)
    el.removeEventListener('touchend', end)
  }
}

/** Haptic feedback */
export function vibrate(pattern: number | number[]): void {
  if (typeof navigator !== 'undefined' && 'vibrate' in navigator) {
    try { navigator.vibrate(pattern) } catch {}
  }
}

/** Safe area insets for notched devices */
export function getSafeAreaInsets(): { top: number; bottom: number; left: number; right: number } {
  if (typeof window === 'undefined') return { top: 0, bottom: 0, left: 0, right: 0 }
  const style = getComputedStyle(document.documentElement)
  return {
    top: parseInt(style.getPropertyValue('--sat') || '0'),
    bottom: parseInt(style.getPropertyValue('--sab') || '0'),
    left: parseInt(style.getPropertyValue('--sal') || '0'),
    right: parseInt(style.getPropertyValue('--sar') || '0'),
  }
}
