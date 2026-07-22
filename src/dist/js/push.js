// Push Notifications module for Proxi Messenger
// Uses Web Push API + Service Worker

const PushNotifications = (() => {
  const VAPID_PUBLIC_KEY = ''; // Will be generated on first use
  let subscription = null;

  // Check if push notifications are supported
  function isSupported() {
    return 'PushManager' in window && 'serviceWorker' in navigator && 'Notification' in window;
  }

  // Request permission and subscribe
  async function requestPermission() {
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

      // Save subscription to server
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

  // Show local notification (fallback when no push server)
  function showLocal(title, body, icon) {
    if (Notification.permission === 'granted') {
      new Notification(title, { body, icon: icon || '/favicon.ico', tag: 'proxi-message' });
    }
  }

  // Get current subscription
  function getSubscription() {
    return subscription;
  }

  return { isSupported, requestPermission, showLocal, getSubscription };
})();
