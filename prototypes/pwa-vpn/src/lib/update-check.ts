/**
 * INF-003: PWA update prompt — Service Worker version check + 'Update' button
 */

let currentVersion = ''
let updateAvailable = false
let onUpdateCallback: (() => void) | null = null

export function getUpdateState() {
  return { updateAvailable, currentVersion }
}

export function onUpdateReady(cb: () => void) {
  onUpdateCallback = cb
}

export async function checkForUpdate(): Promise<void> {
  if (!('serviceWorker' in navigator)) return
  try {
    const reg = await navigator.serviceWorker.getRegistration()
    if (!reg) return
    await reg.update()
    // Listen for new SW
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      updateAvailable = true
      onUpdateCallback?.()
    })
  } catch {}
}

export function applyUpdate(): void {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.controller?.postMessage('SKIP_WAITING')
    setTimeout(() => location.reload(), 1000)
  }
}

// Poll SW version periodically
export async function getSWVersion(): Promise<string> {
  if (!('serviceWorker' in navigator)) return 'unknown'
  try {
    const reg = await navigator.serviceWorker.ready
    const channel = new MessageChannel()
    return new Promise<string>((resolve) => {
      const timeout = setTimeout(() => resolve('unknown'), 3000)
      channel.port1.onmessage = (e) => {
        clearTimeout(timeout)
        resolve(e.data?.version || 'unknown')
      }
      reg.active?.postMessage('GET_VERSION', [channel.port2])
    })
  } catch {
    return 'unknown'
  }
}
