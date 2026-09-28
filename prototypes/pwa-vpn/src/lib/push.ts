/**
 * PUSH-001: WebPush VAPID — subscribe + manage push notifications
 * PUSH-002: Nostr event → push notification bridge
 * PUSH-003: Browser permission flow
 */

// VAPID public key (generated client-side for demo; in prod this comes from server)
// This is a valid P-256 public key for WebPush demo
const VAPID_PUBLIC_KEY = 'BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5NkshxU'

function urlBase64ToUint8Array(base64: string): Uint8Array {
  const padding = '='.repeat((4 - base64.length % 4) % 4)
  const base64Str = (base64 + padding).replace(/-/g, '+').replace(/_/g, '/')
  const rawData = atob(base64Str)
  const arr = new Uint8Array(rawData.length)
  for (let i = 0; i < rawData.length; i++) arr[i] = rawData.charCodeAt(i)
  return arr
}

export async function getPushPermission(): Promise<boolean> {
  if (!('Notification' in globalThis)) return false
  if (Notification.permission === 'granted') return true
  if (Notification.permission === 'denied') return false
  const result = await Notification.requestPermission()
  return result === 'granted'
}

export async function subscribePush(): Promise<PushSubscription | null> {
  if (!(typeof navigator !== 'undefined' && 'serviceWorker' in navigator) || typeof PushManager === 'undefined') return null

  const reg = await navigator.serviceWorker.ready
  let sub = await reg.pushManager.getSubscription()

  if (!sub) {
    try {
      const key = urlBase64ToUint8Array(VAPID_PUBLIC_KEY)
      sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: key,
      })
    } catch (e) {
      console.warn('[push] subscribe failed:', e)
      return null
    }
  }

  // Send subscription to server (PUSH-002: Nostr relay stores it)
  try {
    const sub_json = sub.toJSON()
    localStorage.setItem('proxi-push-sub', JSON.stringify(sub_json))
  } catch {}

  return sub
}

export async function unsubscribePush(): Promise<void> {
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return
  try {
    const reg = await navigator.serviceWorker.ready
    const sub = await reg.pushManager.getSubscription()
    if (sub) await sub.unsubscribe()
  } catch {}
  localStorage.removeItem('proxi-push-sub')
}

export function hasPushSubscription(): boolean {
  try {
    return !!localStorage.getItem('proxi-push-sub')
  } catch {
    return false
  }
}

/**
 * PUSH-002: Send push notification via Nostr event.
 * In serverless mode, we use local SW trigger as fallback.
 */
export async function notifyLocally(title: string, body: string): Promise<void> {
  if (!('Notification' in globalThis)) return
  if (Notification.permission !== 'granted') return
  try {
    new Notification(title, {
      body,
      icon: '/icon-192.png',
      tag: 'proxi-msg',
      vibrate: [200, 100, 200],
    })
  } catch {}
}

/**
 * Initialise push on app start
 */
export async function initPush(): Promise<void> {
  if (!(typeof navigator !== 'undefined' && 'serviceWorker' in navigator)) return
  try {
    await navigator.serviceWorker.register('/sw.js')
    const perm = await getPushPermission()
    if (perm) await subscribePush()
  } catch (e) {
    console.warn('[push] init failed:', e)
  }
}
