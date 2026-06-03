// Push Notifications module
// Uses Web Push API + Service Worker

const VAPID_PUBLIC_KEY = '';
let subscription = null;

export function isSupported() {
  return 'PushManager' in window && 'serviceWorker' in navigator && 'Notification' in window;
}

export async function requestPermission() {
  if (!isSupported()) {
    console.warn('Push notifications not supported');
    return null;
  }

  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    console.warn('Push notification permission denied');
    return null;
  }

  try {
    const reg = await navigator.serviceWorker.ready;
    subscription = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: VAPID_PUBLIC_KEY || undefined,
    });

    await fetch('/api/push/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(subscription.toJSON()),
    });

    console.log('Push subscription registered');
    return subscription;
  } catch (e) {
    console.warn('Push subscription failed:', e);
    return null;
  }
}

export function showLocal(title, body, icon) {
  if (Notification.permission === 'granted') {
    new Notification(title, { body, icon: icon || '/favicon.ico', tag: 'proxi-message' });
  }
}

export function getSubscription() {
  return subscription;
}
