/**
 * SL-051 Web Push client (Phase 2).
 * Primary offline channel remains NIP-59 gift wrap / Nostr.
 * This module registers browser PushSubscription when VAPID available.
 */

export type PushServerConfig = {
  enabled: boolean
  vapidPublic?: string
  phase: string
  note?: string
}

export async function fetchPushConfig(): Promise<PushServerConfig> {
  const r = await fetch('/api/push/config')
  if (!r.ok) throw new Error('push config ' + r.status)
  return r.json()
}

export async function enableDevVapid(): Promise<PushServerConfig> {
  const r = await fetch('/api/push/config?dev=1', { method: 'POST' })
  if (!r.ok) throw new Error('dev vapid ' + r.status)
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

/** Best-effort browser subscribe. Fails closed on insecure origin / denied permission. */
export async function subscribeWebPush(): Promise<{ ok: boolean; reason?: string }> {
  if (typeof window === 'undefined' || !('serviceWorker' in navigator) || !('PushManager' in window)) {
    return { ok: false, reason: 'push_unsupported' }
  }
  let cfg = await fetchPushConfig()
  if (!cfg.vapidPublic) {
    cfg = await enableDevVapid()
  }
  if (!cfg.vapidPublic) return { ok: false, reason: 'no_vapid' }

  const perm = await Notification.requestPermission()
  if (perm !== 'granted') return { ok: false, reason: 'permission_' + perm }

  const reg = await navigator.serviceWorker.ready
  const sub = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(cfg.vapidPublic) as BufferSource,
  })
  const r = await fetch('/api/push/subscribe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub.toJSON()),
  })
  if (!r.ok) return { ok: false, reason: 'subscribe_http_' + r.status }
  return { ok: true }
}
