<script lang="ts">
  import { nostrVPN, type VPNEvent } from '../lib/nostr-vpn'
  import { wtVPN } from '../lib/webtransport'
  import { getPubkey, getSeckey } from '../lib/api'
  import * as secp from '@noble/secp256k1'

  let showInviteModal = $state(false)
  let showRequestModal = $state(false)
  let incomingEvent = $state<VPNEvent | null>(null)
  let friendId = $state('')
  let vpnStatus = $state<'off' | 'sharing' | 'connected'>('off')
  let statusText = $state('VPN выключен')
  let log = $state<string[]>([])
  let lastTunnelIp = $state('')
  let handlingIncoming = $state(false)
  let dismissedFrom = $state<Record<string, number>>({})

  function addLog(msg: string) {
    const t = new Date().toLocaleTimeString()
    log = [`${t}: ${msg}`, ...log].slice(0, 8)
  }

  function hexToBytes(hex: string): Uint8Array {
    const h = hex.length % 2 ? '0' + hex : hex
    const out = new Uint8Array(h.length / 2)
    for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16)
    return out
  }
  function bytesToHex(b: Uint8Array): string {
    return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
  }
  function mySigningPubkey(): string {
    const sk = (getSeckey() || '').slice(0, 64)
    if (sk.length === 64) return bytesToHex(secp.schnorr.getPublicKey(hexToBytes(sk)))
    return getPubkey()
  }
  /** Accept hex pubkey or demo 1111.../2222... seckey-as-id */
  function normalizePeerId(raw: string): string {
    const s = raw.trim().toLowerCase().replace(/^0x/, '')
    if (/^[0-9a-f]{64}$/.test(s)) {
      // If it's a demo seckey (111.. or 222..), derive pubkey
      try {
        return bytesToHex(secp.schnorr.getPublicKey(hexToBytes(s)))
      } catch {
        return s
      }
    }
    return s
  }

  async function authHeaders(): Promise<Record<string, string>> {
    const token = localStorage.getItem('proxi_token') || ''
    return { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token }
  }

  async function startWTServer(): Promise<{ wtAddr: string; certHash: string }> {
    const h = await authHeaders()
    const start = await fetch('/api/vpn/wt/start', { method: 'POST', headers: h, body: '{}' })
    if (!start.ok) throw new Error('WT start HTTP ' + start.status)
    const stats = await start.json()
    const certHash = stats.certHash || ''
    // Prefer LAN host + actual WT port from addr
        // Prefer page hostname (LAN/localhost). Never publish 0.0.0.0 or bare [::].
    // Use window.location.hostname so friends on same LAN can connect.
    let host = window.location.hostname || '127.0.0.1'
    if (host === '[::]' || host === '::' || host === '0.0.0.0') host = '127.0.0.1'
    let port = '4433'
    if (typeof stats.addr === 'string' && stats.addr) {
      const m = String(stats.addr).match(/:(\d+)$/)
      if (m) port = m[1]
    }
    const wtAddr = `${host}:${port}`
    return { wtAddr, certHash }
  }

  async function ensureNostr() {
    await nostrVPN.init(mySigningPubkey())
  }

  // "Give VPN to friend"
  async function giveVPN() {
    if (!friendId.trim()) { addLog('Введите ID друга'); return }
    try {
      await ensureNostr()
      const { wtAddr, certHash } = await startWTServer()
      const peer = normalizePeerId(friendId)
      const id = await nostrVPN.inviteFriend(peer, wtAddr, certHash)
      vpnStatus = 'sharing'
      statusText = `Раздаю VPN · ${wtAddr}`
      addLog(`WT ${wtAddr} hash=${certHash.slice(0, 12)}…`)
      addLog(`Инвайт OK id=${id.slice(0, 12)}… → ${peer.slice(0, 12)}…`)
      // Tell SW we are exit node (optional)
      navigator.serviceWorker?.controller?.postMessage({ type: 'VPN_EXIT_ON', wtAddr, certHash })
      showInviteModal = false
    } catch (e) {
      addLog('Ошибка: ' + (e as Error).message)
    }
  }

  async function requestVPN() {
    if (!friendId.trim()) { addLog('Введите ID друга'); return }
    try {
      await ensureNostr()
      const peer = normalizePeerId(friendId)
      const id = await nostrVPN.requestVPN(peer)
      addLog(`Запрос OK id=${id.slice(0, 12)}… → ${peer.slice(0, 12)}…`)
      showRequestModal = false
    } catch (e) {
      addLog('Ошибка: ' + (e as Error).message)
    }
  }

  async function acceptVPN() {
    if (!incomingEvent || handlingIncoming) return
    handlingIncoming = true
    const ev = incomingEvent
    incomingEvent = null
    try {
      await ensureNostr()
      if (ev.type === 'vpn-request') {
        const { wtAddr, certHash } = await startWTServer()
        await nostrVPN.acceptVPN(ev.from, wtAddr, certHash)
        vpnStatus = 'sharing'
        statusText = `Раздаю VPN · ${wtAddr}`
        addLog('Запрос принят, раздаю ' + wtAddr)
      } else {
        await nostrVPN.acceptVPN(ev.from)
        if (!ev.wtAddr) throw new Error('В инвайте нет wtAddr')
        addLog('WT connect → ' + ev.wtAddr)
        await wtVPN.connect(ev.wtAddr, ev.wtCertHash)
        navigator.serviceWorker?.controller?.postMessage({
          type: 'VPN_CLIENT_ON',
          wtAddr: ev.wtAddr,
          certHash: ev.wtCertHash || '',
        })
        // Stable probe: try api.ipify.org, fallback to stats from server
        let probeOk = false
        let probeIp = ''
        for (const probeUrl of ['http://api.ipify.org', 'http://ifconfig.me/ip']) {
          try {
            const body = (await wtVPN.fetchHTTP(probeUrl)).trim()
            if (body && /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(body)) {
              probeOk = true
              probeIp = body
              break
            }
          } catch {
            // try next
          }
        }
        if (probeOk) {
          lastTunnelIp = probeIp
          addLog('✅ Туннель IP: ' + probeIp)
        } else {
          // Fallback: check server-side stats for bytes transferred
          try {
            const token = localStorage.getItem('proxi_token')
            const r = await fetch('/api/vpn/wt/stats', {
              headers: { Authorization: 'Bearer ' + (token || '') },
            })
            const stats = await r.json()
            if (stats.bytesIn > 0 || stats.bytesOut > 0) {
              lastTunnelIp = 'туннель активен'
              addLog(`✅ Туннель активен: ↓${stats.bytesIn}B ↑${stats.bytesOut}B`)
            } else {
              addLog('Туннель CONNECT OK (нет данных)')
            }
          } catch {
            addLog('Туннель CONNECT OK')
          }
        }
        vpnStatus = 'connected'
        statusText = 'Подключён к VPN друга'
        addLog('WebTransport connected')
      }
      dismissedFrom = { ...dismissedFrom, [ev.from]: Date.now() }
    } catch (e) {
      addLog('Ошибка accept: ' + (e as Error).message)
      dismissedFrom = { ...dismissedFrom, [ev.from]: Date.now() }
    } finally {
      handlingIncoming = false
    }
  }

  async function rejectVPN() {
    if (!incomingEvent) return
    const ev = incomingEvent
    incomingEvent = null
    try {
      await ensureNostr()
      await nostrVPN.rejectVPN(ev.from)
    } catch {}
    dismissedFrom = { ...dismissedFrom, [ev.from]: Date.now() }
    addLog('Отклонено')
  }

  async function disconnectVPN() {
    try { await wtVPN.disconnect() } catch {}
    navigator.serviceWorker?.controller?.postMessage({ type: 'VPN_OFF' })
    try {
      const h = await authHeaders()
      await fetch('/api/vpn/wt/stop', { method: 'POST', headers: h, body: '{}' })
    } catch {}
    vpnStatus = 'off'
    statusText = 'VPN выключен'
    lastTunnelIp = ''
    addLog('Отключён')
  }


  // ── SW Tunnel Handler ──
  // SW intercepts external fetches, asks page to route through WT CONNECT tunnel
  async function handleTunnelFetch(event: MessageEvent) {
    const data = (event as any).data || {}
    if (data.type !== 'TUNNEL_FETCH') return
    const port = (event as any).ports?.[0]
    if (!port) return
    const { target, method, path, host, headers } = data
    try {
      const { readable, writable } = await wtVPN.openConnect(target)
      const writer = writable.getWriter()
      const enc = new TextEncoder()
      let req = `${method} ${path} HTTP/1.1\r\nHost: ${host}\r\n`
      for (const [k, v] of Object.entries(headers || {})) {
        if (k.toLowerCase() === 'host') continue
        req += `${k}: ${(v as string)}\r\n`
      }
      req += 'Connection: close\r\n\r\n'
      await writer.write(enc.encode(req))
      try { await writer.close() } catch {}
      const reader = readable.getReader()
      const chunks: Uint8Array[] = []
      let total = 0
      const deadline = Date.now() + 10000
      while (Date.now() < deadline) {
        const { value, done } = await Promise.race([
          reader.read(),
          new Promise<{ done: true; value: undefined }>((r) => setTimeout(() => r({ done: true, value: undefined }), 3000)),
        ]) as any
        if (done) break
        if (value) {
          chunks.push(value)
          total += value.byteLength
          if (total > 1048576) break
        }
      }
      try { reader.releaseLock() } catch {}
      const resp = new Uint8Array(total)
      let off = 0
      for (const c of chunks) { resp.set(c, off); off += c.byteLength }
      port.postMessage({ type: 'COMPLETE', body: resp }, [resp.buffer])
    } catch (e) {
      port.postMessage({ type: 'ERROR', error: (e as Error).message })
    }
  }

  function registerSWTunnel() {
    if (typeof navigator === 'undefined' || !navigator.serviceWorker) return
    if (navigator.serviceWorker.controller) {
      navigator.serviceWorker.addEventListener('message', handleTunnelFetch)
      const mc = new MessageChannel()
      navigator.serviceWorker.controller.postMessage({ type: 'VPN_PAGE_READY' }, [mc.port2])
      console.log('[VPN] SW tunnel provider registered')
    }
  }

  let unsubVPN: (() => void) | null = null
  $effect(() => {
    const token = localStorage.getItem('proxi_token')
    if (!token) return
    let cancelled = false
    ;(async () => {
      try {
        await ensureNostr()
        if (cancelled) return
        unsubVPN = nostrVPN.onVPNEvent((event) => {
          if (event.type === 'vpn-invite' || event.type === 'vpn-request') {
            if (handlingIncoming || vpnStatus !== 'off') return
            if (dismissedFrom[event.from] && Date.now() - dismissedFrom[event.from] < 60000) return
            if (incomingEvent) return // one at a time
            incomingEvent = event
            addLog(`Входящий ${event.type} от ${event.from.slice(0, 12)}…`)
          } else if (event.type === 'vpn-accept') {
            addLog(`Accept от ${event.from.slice(0, 12)}… addr=${event.wtAddr || '—'}`)
          } else if (event.type === 'vpn-reject') {
            addLog(`Reject от ${event.from.slice(0, 12)}…`)
          }
        })
        addLog('Nostr VPN signaling ON')
      } catch (e) {
        addLog('Nostr init: ' + (e as Error).message)
      }
    })()
    return () => {
      cancelled = true
      if (unsubVPN) unsubVPN()
    }
  })
</script>

<div class="vpn-product" data-testid="vpn-product">
  {#if vpnStatus === 'off'}
    <div class="vpn-buttons">
      <button class="vpn-btn share" data-testid="vpn-give" onclick={() => showInviteModal = true}>
        📡 Дать VPN другу
      </button>
      <button class="vpn-btn request" data-testid="vpn-request" onclick={() => showRequestModal = true}>
        🤝 Запросить VPN
      </button>
    </div>
  {:else}
    <div class="vpn-status" class:sharing={vpnStatus === 'sharing'} class:connected={vpnStatus === 'connected'}>
      <span class="status-dot"></span>
      <span>{statusText}</span>
      {#if lastTunnelIp}<span class="ip">IP {lastTunnelIp}</span>{/if}
      <button class="disconnect-btn" data-testid="vpn-off" onclick={disconnectVPN}>Отключить</button>
    </div>
  {/if}

  {#if log.length > 0}
    <div class="vpn-log" data-testid="vpn-log">
      {#each log as entry}
        <div class="log-entry">{entry}</div>
      {/each}
    </div>
  {/if}
</div>

{#if showInviteModal}
  <div class="modal-overlay" onclick={() => showInviteModal = false}>
    <div class="modal" onclick={(e) => e.stopPropagation()} data-testid="vpn-invite-modal">
      <h3>📡 Поделиться VPN с другом</h3>
      <p>ID друга (hex pubkey или demo-ключ 111…/222…):</p>
      <input type="text" placeholder="pubkey друга..." bind:value={friendId} data-testid="vpn-friend-id" />
      <div class="modal-buttons">
        <button class="cancel" onclick={() => showInviteModal = false}>Отмена</button>
        <button class="confirm" data-testid="vpn-share-confirm" onclick={giveVPN}>Раздать</button>
      </div>
    </div>
  </div>
{/if}

{#if showRequestModal}
  <div class="modal-overlay" onclick={() => showRequestModal = false}>
    <div class="modal" onclick={(e) => e.stopPropagation()}>
      <h3>🤝 Запросить VPN у друга</h3>
      <p>ID друга (hex pubkey):</p>
      <input type="text" placeholder="pubkey друга..." bind:value={friendId} />
      <div class="modal-buttons">
        <button class="cancel" onclick={() => showRequestModal = false}>Отмена</button>
        <button class="confirm" onclick={requestVPN}>Запросить</button>
      </div>
    </div>
  </div>
{/if}

{#if incomingEvent}
  <div class="modal-overlay" data-testid="vpn-incoming-modal">
    <div class="modal">
      <h3>{incomingEvent.type === 'vpn-invite' ? '📡 Друг предлагает VPN!' : '🤝 Друг просит VPN'}</h3>
      <p>От: <code>{incomingEvent.from.slice(0, 24)}…</code></p>
      {#if incomingEvent.wtAddr}<p>Exit: <code>{incomingEvent.wtAddr}</code></p>{/if}
      <div class="modal-buttons">
        <button class="cancel" data-testid="vpn-reject" onclick={rejectVPN}>Отклонить</button>
        <button class="confirm" data-testid="vpn-accept" onclick={acceptVPN}>
          {incomingEvent.type === 'vpn-invite' ? 'Принять' : 'Разрешить'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .vpn-product { padding: 12px; }
  .vpn-buttons { display: flex; flex-direction: column; gap: 8px; }
  .vpn-btn { padding: 14px; border: none; border-radius: 12px; font-size: 15px; font-weight: 600; cursor: pointer; font-family: inherit; }
  .vpn-btn.share { background: linear-gradient(135deg, #1e3a5f, #2a6a4a); color: #fff; }
  .vpn-btn.request { background: linear-gradient(135deg, #2a4a6a, #3b5998); color: #fff; }
  .vpn-status { display: flex; align-items: center; gap: 8px; padding: 14px; border-radius: 12px; font-size: 13px; font-weight: 600; flex-wrap: wrap; }
  .vpn-status.sharing { background: #0d2818; color: #6ee7b7; border: 1px solid #1a4a2a; }
  .vpn-status.connected { background: #0d1a28; color: #7dd3fc; border: 1px solid #1a3a5a; }
  .status-dot { width: 10px; height: 10px; border-radius: 50%; background: currentColor; }
  .ip { font-family: monospace; opacity: 0.9; }
  .disconnect-btn { margin-left: auto; background: rgba(255,107,107,0.2); border: 1px solid #ff6b6b33; color: #ff6b6b; padding: 4px 12px; border-radius: 6px; cursor: pointer; font-size: 12px; font-family: inherit; }
  .vpn-log { margin-top: 8px; max-height: 110px; overflow-y: auto; }
  .log-entry { font-size: 11px; color: #6a7a8a; padding: 2px 0; font-family: monospace; }
  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.7); display: flex; align-items: center; justify-content: center; z-index: 1000; padding: 20px; }
  .modal { background: #161b22; border: 1px solid #30363d; border-radius: 16px; padding: 24px; max-width: 400px; width: 100%; }
  .modal h3 { margin: 0 0 8px; font-size: 18px; color: #e0e0e0; }
  .modal p { font-size: 13px; color: #8b949e; margin: 8px 0; }
  .modal input { width: 100%; padding: 10px; background: #0d1117; border: 1px solid #30363d; border-radius: 8px; color: #e0e0e0; font-size: 14px; font-family: monospace; box-sizing: border-box; }
  .modal-buttons { display: flex; gap: 8px; margin-top: 16px; }
  .modal-buttons button { flex: 1; padding: 10px; border: none; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; font-family: inherit; }
  .modal-buttons .cancel { background: #21262d; color: #8b949e; }
  .modal-buttons .confirm { background: #238636; color: #fff; }
</style>
