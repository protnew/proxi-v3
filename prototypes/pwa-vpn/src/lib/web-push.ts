/**
 * SL-051 Web Push client — production VAPID from /api/push/config.
 * Primary offline channel remains NIP-59 gift wrap (kind 1059).
 */

export type PushServerConfig = {
  enabled: boolean
  vapidPublic?: string
  phase: string
  note?: string
  subject?: string
  subscribers?: number
  keysSource?: string
}

export async function fetchPushConfig(): Promise<PushServerConfig> {
  const r = await fetch('/api/push/config')
  if (!r.ok) throw new Error('push config ' + r.status)
  return r.json()
}

/** Ensure server has persisted VAPID keys (idempotent). */
export async function ensureVapid(): Promise<PushServerConfig> {
  const r = await fetch('/api/push/config', { method: 'POST' })
  if (!r.ok) throw new Error('ensure vapid ' + r.status)
  return r.json()
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  const out = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

export async function subscribeWebPush(): Promise<{ ok: boolean; reason?: string; endpoint?: string }> {
  if (typeof window === 'undefined' || !('serviceWorker' in navigator) || !('PushManager' in window)) {
    return { ok: false, reason: 'push_unsupported' }
  }
  let cfg = await fetchPushConfig()
  if (!cfg.vapidPublic) cfg = await ensureVapid()
  if (!cfg.vapidPublic) return { ok: false, reason: 'no_vapid' }

  // Secure context required
  if (!window.isSecureContext && location.hostname !== 'localhost' && location.hostname !== '127.0.0.1') {
    return { ok: false, reason: 'insecure_context' }
  }

  let perm = Notification.permission
  if (perm === 'default') perm = await Notification.requestPermission()
  if (perm !== 'granted') return { ok: false, reason: 'permission_' + perm }

  const reg = await navigator.serviceWorker.ready
  let sub = await reg.pushManager.getSubscription()
  if (!sub) {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(cfg.vapidPublic) as BufferSource,
    })
  }
  const r = await fetch('/api/push/subscribe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub.toJSON()),
  })
  if (!r.ok) return { ok: false, reason: 'subscribe_http_' + r.status }
  return { ok: true, endpoint: sub.endpoint }
}

export async function sendTestPush(title = 'Proxi', body = 'Test push'): Promise<unknown> {
  const r = await fetch('/api/push/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, body }),
  })
  return r.json()
}
