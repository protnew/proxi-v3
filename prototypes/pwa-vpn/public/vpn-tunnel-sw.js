// VPN tunnel fetch assist — works with page-controlled WebTransport via postMessage.
// Full system VPN is impossible in pure PWA; we intercept opted-in fetches.
let vpnMode = 'off' // off | client
let wtAddr = ''
let certHash = ''

self.addEventListener('message', (event) => {
  const data = event.data || {}
  if (data.type === 'VPN_CLIENT_ON') {
    vpnMode = 'client'
    wtAddr = data.wtAddr || ''
    certHash = data.certHash || ''
    console.log('[vpn-sw] CLIENT_ON', wtAddr)
  } else if (data.type === 'VPN_EXIT_ON') {
    vpnMode = 'exit'
    wtAddr = data.wtAddr || ''
    console.log('[vpn-sw] EXIT_ON', wtAddr)
  } else if (data.type === 'VPN_OFF') {
    vpnMode = 'off'
    wtAddr = ''
    certHash = ''
    console.log('[vpn-sw] OFF')
  } else if (data.type === 'VPN_STATUS') {
    event.source && event.source.postMessage({ type: 'VPN_STATUS', vpnMode, wtAddr })
  }
})

// Mark external http(s) requests when VPN client is on — actual tunnel is in page
// (WebTransport is more reliable in window than SW). SW adds header for debug.
self.addEventListener('fetch', (event) => {
  if (vpnMode !== 'client') return
  const url = new URL(event.request.url)
  if (url.origin === self.location.origin) return
  // We cannot open WebTransport CONNECT for arbitrary HTTPS from SW portably.
  // Annotate only; page uses wtVPN.fetchHTTP for proof path.
  // Future: message page to proxy.
})
