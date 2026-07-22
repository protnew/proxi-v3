/**
 * Tauri API bridge — desktop-specific features
 * Falls back gracefully when not running in Tauri
 */

const isTauri = typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window

export function getIsDesktop(): boolean {
  return isTauri
}

/**
 * Get VPN status from Tauri backend
 */
export async function getDesktopVpnStatus(): Promise<{ status: string; peers: number }> {
  if (!isTauri) return { status: 'unavailable', peers: 0 }
  const { invoke } = await import(/* @vite-ignore */ '@tauri-apps/api/core')
  const result = await (invoke as any)('get_vpn_status')
  return JSON.parse(result)
}

/**
 * Start VPN via Tauri backend (WireGuard)
 */
export async function startDesktopVpn(config: string): Promise<string> {
  if (!isTauri) throw new Error('Not running in Tauri')
  const { invoke } = await import(/* @vite-ignore */ '@tauri-apps/api/core')
  return (invoke as any)('start_vpn', { config })
}

/**
 * Stop VPN via Tauri backend
 */
export async function stopDesktopVpn(): Promise<string> {
  if (!isTauri) throw new Error('Not running in Tauri')
  const { invoke } = await import(/* @vite-ignore */ '@tauri-apps/api/core')
  return (invoke as any)('stop_vpn')
}

/**
 * Get system info
 */
export async function getSystemInfo(): Promise<{ os: string; arch: string }> {
  if (!isTauri) return { os: navigator.platform, arch: 'unknown' }
  const { invoke } = await import(/* @vite-ignore */ '@tauri-apps/api/core')
  const result = await (invoke as any)('get_system_info')
  return JSON.parse(result)
}

/**
 * Show desktop notification
 */
export async function showNotification(title: string, body: string): Promise<void> {
  if (isTauri) {
    try {
      const { sendNotification } = await import(/* @vite-ignore */ '@tauri-apps/plugin-notification')
      sendNotification({ title, body })
      return
    } catch {}
  }
  // Fallback to web notification
  if ('Notification' in window && Notification.permission === 'granted') {
    new Notification(title, { body })
  }
}
