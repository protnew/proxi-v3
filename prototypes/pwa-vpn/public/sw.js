/**
 * Service Worker — Proxi Messenger
 * PUSH-001/002/003: WebPush + update prompt
 */
const SW_VERSION = '2026-08-11-v1'
const CACHE_NAME = 'proxi-v' + SW_VERSION
const APP_SHELL = [
  '/',
  '/index.html',
  '/manifest.json',
]

// INSTALL — precache app shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then(cache => cache.addAll(APP_SHELL).catch(() => {}))
  )
  self.skipWaiting()
})

// ACTIVATE — clean old caches + claim clients
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE_NAME).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  )
})

// FETCH — network-first for navigation, cache-first for assets
self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return

  // Navigation requests → network first, fallback to cache
  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req).then(res => {
        const clone = res.clone()
        caches.open(CACHE_NAME).then(cache => cache.put('/', clone)).catch(() => {})
        return res
      }).catch(() => caches.match('/').then(r => r || caches.match('/index.html')))
    )
    return
  }

  // Static assets → cache first
  event.respondWith(
    caches.match(req).then(cached => cached || fetch(req).then(res => {
      if (res.ok && req.url.startsWith(self.location.origin)) {
        const clone = res.clone()
        caches.open(CACHE_NAME).then(cache => cache.put(req, clone)).catch(() => {})
      }
      return res
    }).catch(() => cached))
  )
})

// PUSH — WebPush notification (PUSH-001/002)
self.addEventListener('push', (event) => {
  let data = { title: 'Новое сообщение', body: 'У вас новое сообщение' }
  try {
    if (event.data) data = event.data.json()
  } catch {
    if (event.data) data.body = event.data.text()
  }

  const options = {
    body: data.body,
    icon: '/icon-192.png',
    badge: '/icon-192.png',
    tag: data.tag || 'proxi-msg',
    renotify: true,
    data: data.data || {},
    vibrate: [200, 100, 200],
    actions: [
      { action: 'open', title: 'Открыть' },
      { action: 'close', title: 'Закрыть' },
    ],
  }

  event.waitUntil(
    self.registration.showNotification(data.title || 'Proxi', options)
  )
})

// NOTIFICATION CLICK
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  if (event.action === 'close') return

  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(clients => {
      for (const client of clients) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          return client.focus()
        }
      }
      if (self.clients.openWindow) return self.clients.openWindow('/')
    })
  )
})

// MESSAGE — update prompt (INF-003)
self.addEventListener('message', (event) => {
  if (event.data === 'SKIP_WAITING') self.skipWaiting()
  if (event.data === 'GET_VERSION') {
    event.ports[0].postMessage({ version: SW_VERSION })
  }
})

// CONTROLLER CHANGE — notify page about update
self.addEventListener('controllerchange', () => {})
