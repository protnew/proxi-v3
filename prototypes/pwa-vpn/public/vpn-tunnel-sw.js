// vpn-tunnel-sw.js — Service Worker transparent proxy
// Intercepts external http(s) fetch requests and routes them through
// the page's WebTransport CONNECT tunnel (exit-node on friend's side).
//
// Architecture: SW cannot open WebTransport directly in all browsers reliably,
// so SW delegates CONNECT to the page via MessageChannel, receives raw bytes back.

let vpnMode = 'off'
let wtAddr = ''
let certHash = ''
let pagePort = null // MessageChannel port to the page

self.addEventListener('message', (event) => {
  const data = event.data || {}
  switch (data.type) {
    case 'VPN_CLIENT_ON':
      vpnMode = 'client'
      wtAddr = data.wtAddr || ''
      certHash = data.certHash || ''
      console.log('[vpn-sw] CLIENT_ON', wtAddr)
      break
    case 'VPN_EXIT_ON':
      vpnMode = 'exit'
      wtAddr = data.wtAddr || ''
      console.log('[vpn-sw] EXIT_ON', wtAddr)
      break
    case 'VPN_OFF':
      vpnMode = 'off'
      wtAddr = ''
      certHash = ''
      console.log('[vpn-sw] OFF')
      break
    case 'VPN_STATUS':
      if (event.source) {
        event.source.postMessage({ type: 'VPN_STATUS', vpnMode, wtAddr, certHash })
      }
      break
    case 'VPN_PAGE_READY':
      // Page registers itself as the WT tunnel provider
      if (event.ports && event.ports[0]) {
        pagePort = event.ports[0]
        pagePort.onmessage = null // page sends, we already have the port
        console.log('[vpn-sw] Page tunnel provider registered')
      }
      break
  }
})

// Pending request map: requestId → { resolve, reject }
const pending = new Map()
let reqCounter = 0

self.addEventListener('fetch', (event) => {
  if (vpnMode !== 'client') return
  const url = new URL(event.request.url)
  // Don't intercept same-origin (app assets, API)
  if (url.origin === self.location.origin) return
  // Only intercept http/https
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return

  event.respondWith(proxyViaTunnel(event.request))
})

async function proxyViaTunnel(request) {
  if (!pagePort) {
    // Fallback: direct fetch if no page tunnel registered
    return fetch(request)
  }

  const url = new URL(request.url)
  const port = url.port || (url.protocol === 'https:' ? '443' : '80')
  const target = `${url.hostname}:${port}`
  const reqId = ++reqCounter

  // Serialize request body
  let bodyBytes = null
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    try {
      const ab = await request.arrayBuffer()
      bodyBytes = new Uint8Array(ab)
    } catch {
      bodyBytes = null
    }
  }

  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      if (pending.has(reqId)) {
        pending.delete(reqId)
        reject(new Error('VPN tunnel timeout'))
      }
    }, 15000)

    pending.set(reqId, { resolve, reject, timeout })

    // Ask the page to open a CONNECT tunnel and send HTTP request
    const mc = new MessageChannel()
    mc.port1.onmessage = (ev) => {
      const msg = ev.data
      if (msg.type === 'CHUNK') {
        // Accumulated in port2 side; page sends COMPLETE when done
      } else if (msg.type === 'COMPLETE') {
        clearTimeout(timeout)
        pending.delete(reqId)
        mc.port1.close()
        mc.port2.close()
        const respBytes = new Uint8Array(msg.body || [])
        const respText = new TextDecoder().decode(respBytes)
        // Parse HTTP response from raw bytes
        const headerEnd = respText.indexOf('\r\n\r\n')
        let status = 200
        let headers = {}
        let bodyText = ''
        if (headerEnd >= 0) {
          const headerLines = respText.slice(0, headerEnd).split('\r\n')
          const statusLine = headerLines[0]
          const m = statusLine.match(/HTTP\/\d\.\d (\d+)/)
          if (m) status = parseInt(m[1])
          for (let i = 1; i < headerLines.length; i++) {
            const idx = headerLines[i].indexOf(':')
            if (idx > 0) {
              const k = headerLines[i].slice(0, idx).trim()
              const v = headerLines[i].slice(idx + 1).trim()
              // Skip transfer-encoding/content-length (we handle raw body)
              if (k.toLowerCase() === 'transfer-encoding') continue
              if (k.toLowerCase() === 'content-length') continue
              headers[k] = v
            }
          }
          bodyText = respText.slice(headerEnd + 4)
          const bodyBytesParsed = new TextEncoder().encode(bodyText)
          resolve(new Response(bodyBytesParsed, { status, headers }))
        } else {
          resolve(new Response(respBytes, { status: 200 }))
        }
      } else if (msg.type === 'ERROR') {
        clearTimeout(timeout)
        pending.delete(reqId)
        mc.port1.close()
        mc.port2.close()
        reject(new Error(msg.error || 'tunnel error'))
      }
    }

    pagePort.postMessage({
      type: 'TUNNEL_FETCH',
      reqId,
      target,
      method: request.method,
      path: (url.pathname || '/') + (url.search || ''),
      host: url.host,
      headers: Object.fromEntries(request.headers.entries()),
      hasBody: !!bodyBytes,
    }, [mc.port2])
  })
}
