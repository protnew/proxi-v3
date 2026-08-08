<script lang="ts">
  import { nostrVPN, type VPNEvent } from '../lib/nostr-vpn'
  import { rtcVPN } from '../lib/webrtc-vpn'
  import { onMount } from 'svelte'
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
  let turnStatus = $state<string>('checking…')
  let amneziaStatus = $state<string>('')
  let dismissedFrom = $state<Record<string, number>>({})

  onMount(async () => {
    turnStatus = '4 STUN (Google+CF), P2P 85%. Nostr fallback — Phase 1.5'
    amneziaStatus = ''
  })

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

  // "Give VPN to friend" — Alice becomes exit node, waits for friend's WebRTC offer
  async function giveVPN() {
    if (!friendId.trim()) { addLog('Введите ID друга'); return }
    try {
      await ensureNostr()
      const peer = normalizePeerId(friendId)
      const id = await nostrVPN.inviteFriend(peer, '', '')
      vpnStatus = 'sharing'
      statusText = 'Раздаю VPN · жду подключения друга'
      addLog('Инвайт отправлен, готов как exit node (WebRTC)')
      addLog('Инвайт OK id=' + id.slice(0, 12) + '…')
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
        // Friend wants VPN from us — we become exit node
        await nostrVPN.acceptVPN(ev.from, '', '')
        vpnStatus = 'sharing'
        statusText = 'Раздаю VPN · жду WebRTC offer'
        addLog('Запрос принят, готов как exit node (WebRTC)')
      } else if (ev.type === 'vpn-invite') {
        // Friend offers VPN — we are caller, create WebRTC offer
        addLog('WebRTC: создаю offer для exit node ' + ev.from.slice(0, 12) + '…')
        const { sdp, gatherIce } = await rtcVPN.createOffer()
        const iceCandidates: string[] = []
        await gatherIce((c) => { if (c !== 'END') iceCandidates.push(c) })
        await nostrVPN.sendRTCOffer(ev.from, sdp, iceCandidates)
        addLog('WebRTC offer отправлен (' + iceCandidates.length + ' ICE candidates)')
        vpnStatus = 'connecting'
        statusText = 'WebRTC connecting…'
        // Wait for answer via Nostr
        const answerTimeout = setTimeout(() => {
          if (vpnStatus === 'connecting') {
            addLog('WebRTC: таймаут ожидания answer')
            vpnStatus = 'off'
            statusText = 'VPN выключен'
          }
        }, 30000)
        rtcAnswerResolver = (sdp: string, ice: string[]) => {
          clearTimeout(answerTimeout)
          applyRTCAnswer(sdp, ice, ev.from)
        }
      }
      dismissedFrom = { ...dismissedFrom, [ev.from]: Date.now() }
    } catch (e) {
      addLog('Ошибка accept: ' + (e as Error).message)
      dismissedFrom = { ...dismissedFrom, [ev.from]: Date.now() }
    } finally {
      handlingIncoming = false
    }
  }

  let rtcAnswerResolver: ((sdp: string, ice: string[]) => void) | null = null

  async function applyRTCAnswer(sdp: string, ice: string[], from: string) {
    try {
      await rtcVPN.applyAnswer(sdp, ice)
      await rtcVPN.waitForOpen(15000)
      // Probe tunnel: HTTP first, then HTTPS
      let tunnelIp = ''
      try {
        const httpBody = await rtcVPN.fetchHTTP('http://api.ipify.org')
        const ip = httpBody.trim()
        if (ip && /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(ip)) tunnelIp = ip
      } catch {}
      if (!tunnelIp) {
        try {
          const httpsBody = await rtcVPN.fetchHTTPS('https://api.ipify.org')
          const ip2 = httpsBody.trim()
          if (ip2 && /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(ip2)) {
            tunnelIp = ip2
            addLog('HTTPS tunnel OK')
          }
        } catch {}
      }
      if (tunnelIp) {
        lastTunnelIp = tunnelIp
        addLog('WebRTC P2P connected! Exit IP: ' + tunnelIp)
      } else {
        addLog('WebRTC connected (IP probe empty)')
      }
      vpnStatus = 'connected'
      statusText = 'VPN подключён (WebRTC P2P)'
    } catch (e) {
      addLog('WebRTC answer failed: ' + (e as Error).message)
      vpnStatus = 'off'
      statusText = 'VPN выключен'
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
    try { await rtcVPN.disconnect() } catch {}
    navigator.serviceWorker?.controller?.postMessage({ type: 'VPN_OFF' })
    vpnStatus = 'off'
    statusText = 'VPN выключен'
    lastTunnelIp = ''
    addLog('WebRTC отключён')
  }


  // SW tunnel handler — routes fetches through WebRTC DataChannel
  async function handleTunnelFetch(event: MessageEvent) {
    const data = (event as any).data || {}
    if (data.type !== 'TUNNEL_FETCH') return
    const port = (event as any).ports?.[0]
    if (!port) return
    try {
      const body = await rtcVPN.fetchThroughTunnel(data.target || 'api.ipify.org:80')
      port.postMessage({ type: 'COMPLETE', body: new TextEncoder().encode(body) })
    } catch (e) {
      port.postMessage({ type: 'ERROR', error: (e as Error).message })
    }
  }
  function registerSWTunnel() {
    if (typeof navigator === 'undefined' || !navigator.serviceWorker) return
    if (navigator.serviceWorker.controller) {
      navigator.serviceWorker.addEventListener('message', handleTunnelFetch)
      navigator.serviceWorker.controller.postMessage({ type: 'VPN_PAGE_READY' })
      console.log('[VPN] WebRTC SW tunnel registered')
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
            if (incomingEvent) return
            incomingEvent = event
            addLog('Входящий ' + event.type + ' от ' + event.from.slice(0, 12) + '…')
          } else if (event.type === 'rtc-offer') {
            // We are exit node — create WebRTC answer
            handleRTCOffer(event)
          } else if (event.type === 'rtc-answer') {
            // We are caller — apply answer
            if (rtcAnswerResolver) {
              rtcAnswerResolver(event.rtcSdp || '', event.iceCandidates || [])
              rtcAnswerResolver = null
            }
          } else if (event.type === 'rtc-ice') {
            // Trickle ICE
            if (event.iceCandidates) {
              for (const c of event.iceCandidates) {
                try { rtcVPN['pc']?.addIceCandidate({ candidate: c, sdpMid: '0', sdpMLineIndex: 0 }) } catch {}
              }
            }
          } else if (event.type === 'vpn-accept') {
            addLog('Accept от ' + event.from.slice(0, 12) + '…')
          } else if (event.type === 'vpn-reject') {
            addLog('Reject от ' + event.from.slice(0, 12) + '…')
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
  {#if turnStatus}
    <div class="ice-status">
      <span class="ice-badge" title="NAT traversal method">🧊 {turnStatus}</span>
      {#if amneziaStatus}<span class="ice-badge amnezia" title="DPI obfuscation">🛡️ {amneziaStatus}</span>{/if}
    </div>
  {/if}
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
  .ice-status { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 8px; }
  .ice-badge { font-size: 11px; color: #7dd3fc; background: rgba(125,211,252,0.1); padding: 3px 8px; border-radius: 6px; border: 1px solid rgba(125,211,252,0.2); }
  .ice-badge.amnezia { color: #fbbf24; background: rgba(251,191,36,0.1); border-color: rgba(251,191,36,0.2); }
</style>
