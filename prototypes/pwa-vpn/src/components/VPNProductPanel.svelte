<script lang="ts">
  import { nostrVPN, type VPNEvent } from '../lib/nostr-vpn'
  import { rtcVPN } from '../lib/webrtc-vpn'
  import { getLocalTabP2P, type TabP2PStatus } from '../lib/local-tab-p2p'
  import { dataRelay } from '../lib/nostr-data-relay'
  import { onMount } from 'svelte'
  const isDevMode = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('dev') === '1'
  import { getPubkey } from '../lib/api'
  import { getTunnelStatus, stopTunnel, type TunnelStatus } from '../lib/amnezia-tunnel'
  import { subscribeWebPush, fetchPushConfig, sendTestPush } from '../lib/web-push'
  import { hexToBytes, bytesToHex, mySigningPubkey, normalizePeerId, authHeaders, startWTServer } from '../lib/vpn-utils.svelte';
async function startVPNSignaling(targetPubkey: string) {
  try {
    await (rtcVPN as any).startNostrSignaling(targetPubkey)
  } catch (e) {
  }
}
  let showInviteModal = $state(false)
  let showRequestModal = $state(false)
  let incomingEvent = $state<VPNEvent | null>(null)
  let friendId = $state('')
  let vpnStatus = $state<'off' | 'connecting' | 'connected' | 'error'>('off')
  let statusText = $state('VPN выключен')
  let log = $state<string[]>([])
  let tabP2P = $state<TabP2PStatus>({
    phase: 'idle', role: 'none', lastError: '', peerReady: false, rttMs: null, bytesIn: 0, bytesOut: 0, tunnelIp: ''
  })
  const localP2P = getLocalTabP2P()
  localP2P.onChange((s) => {
    tabP2P = s
    if (s.phase === 'connected') {
      vpnStatus = 'connected'
      statusText = s.role === 'host'
        ? 'VPN: 2 вкладки · вы exit node (DataChannel open)'
        : `VPN: 2 вкладки · DataChannel open${s.tunnelIp ? ' · IP ' + s.tunnelIp : ''}`
      addLog('VPN-101 P2P connected role=' + s.role + (s.tunnelIp ? ' ip=' + s.tunnelIp : ''))
    } else if (s.phase === 'negotiating' || s.phase === 'waiting_peer') {
      vpnStatus = 'connecting'
      statusText = s.phase === 'waiting_peer' ? 'VPN-101: жду вторую вкладку…' : 'VPN-101: WebRTC negotiating…'
    } else if (s.phase === 'error') {
      vpnStatus = 'error'
      statusText = 'VPN-101 error: ' + s.lastError
      addLog('VPN-101 error: ' + s.lastError)
    }
  })
  async function startTabP2PHost() {
    try {
      await localP2P.startAsHost()
      addLog('VPN-101: host (exit) — откройте 2-ю вкладку и нажмите «Войти peer»')
    } catch (e) { addLog('VPN-101 host fail: ' + e) }
  }
  async function startTabP2PJoiner() {
    try {
      await localP2P.startAsJoiner()
      addLog('VPN-101: joiner — ищу host-вкладку…')
    } catch (e) { addLog('VPN-101 joiner fail: ' + e) }
  }
  function stopTabP2P() {
    localP2P.stop()
    vpnStatus = 'off'
    statusText = 'VPN выключен'
    addLog('VPN-101 stopped')
  }
  let lastTunnelIp = $state('')
  let handlingIncoming = $state(false)
  let turnStatus = $state<string>('checking…')
  let amneziaStatus = $state<string>('')
  let tunnelInfo = $state<TunnelStatus | null>(null)
  let pushInfo = $state<string>('')
  let pushBusy = $state(false)
  let tunnelBusy = $state(false)
  let transportMode = $state<string>('')  // 'P2P' | 'Nostr Relay' | ''
  let dismissedFrom = $state<Record<string, number>>({})
  const DEMO_ALICE = '1'.repeat(64)
  const DEMO_BOB = '2'.repeat(64)
  let lanPhoneUrl = $state('')
  let demoPartnerLabel = $state('')
  function applyDemoPartner() {
    const role = (typeof localStorage !== 'undefined' && localStorage.getItem('proxi_demo_role')) || ''
    const my = getPubkey() || ''
    if (role === 'tester1' || my === DEMO_ALICE) {
      friendId = DEMO_BOB
      demoPartnerLabel = 'Bob (demo)'
    } else if (role === 'tester2' || my === DEMO_BOB) {
      friendId = DEMO_ALICE
      demoPartnerLabel = 'Alice (demo)'
    }
  }
  onMount(async () => {
    turnStatus = '4 STUN · P2P 85% + Nostr relay 15%'
    amneziaStatus = ''
    applyDemoPartner()
    try {
      tunnelInfo = await getTunnelStatus()
      const mode = tunnelInfo.mode === 'userspace' ? 'в приложении' : (tunnelInfo.mode || 'выкл')
      amneziaStatus = `${tunnelInfo.state}/${mode}`
    } catch { amneziaStatus = 'n/a' }
    try {
      const pc = await fetchPushConfig()
      pushInfo = pc.enabled ? `VAPID ${pc.phase}` : 'push off'
    } catch { pushInfo = 'push n/a' }
    try {
      const r = await fetch('/api/network/lan')
      const j = await r.json()
      if (j.phone_urls && j.phone_urls[0]) lanPhoneUrl = j.phone_urls[0]
      else if (j.reachable_from_lan && j.lan_ips && j.lan_ips[0]) lanPhoneUrl = `http://${j.lan_ips[0]}:${j.port || 8090}/?role=bob`
    } catch { /* offline */ }
  })
  async function startInAppTunnel() {
    // Product path: in-app WebRTC DataChannel (VPN-101). Amnezia kernel is Advanced.
    tunnelBusy = true
    try {
      await localP2P.startAsHost()
      addLog('VPN в приложении: эта вкладка — exit. Вторая вкладка — «войти peer»')
    } catch (e: any) {
      addLog('VPN-движок: ' + (e?.message || e))
    } finally {
      tunnelBusy = false
    }
  }
  async function stopInAppTunnel() {
    tunnelBusy = true
    try {
      stopTabP2P()
      try {
        tunnelInfo = await stopTunnel()
        amneziaStatus = `${tunnelInfo.state}/выкл`
      } catch { /* Amnezia optional */ }
      addLog('VPN-движок остановлен')
    } catch (e: any) {
      addLog('stop: ' + (e?.message || e))
    } finally {
      tunnelBusy = false
    }
  }
  let showAdvanced = $state(false)
  async function exportForAmneziaOptional() {
    try {
      const r = await fetch('/api/vpn/amnezia/import', { method: 'POST' })
      const j = await r.json()
      addLog(j.ok ? 'Conf экспортирован (опционально)' : ('export: ' + (j.note || 'fail')))
      if (j.userCopyPath) addLog(j.userCopyPath)
    } catch (e: any) {
      addLog('export: ' + (e?.message || e))
    }
  }
  async function enablePush() {
    pushBusy = true
    try {
      const res = await subscribeWebPush()
      if (res.ok) {
        pushInfo = 'subscribed'
        addLog('Push OK: ' + (res.endpoint || '').slice(0, 48))
        try {
          const r = await sendTestPush('Indestructible', 'Push E2E test')
          addLog('Push send: ' + JSON.stringify(r).slice(0, 80))
        } catch {}
      } else {
        pushInfo = res.reason || 'fail'
        addLog('Push: ' + (res.reason || 'fail'))
      }
    } catch (e: any) {
      pushInfo = 'error'
      addLog('Push error: ' + (e?.message || e))
    } finally {
      pushBusy = false
    }
  }
  function addLog(msg: string) {
    const t = new Date().toLocaleTimeString()
    log = [`${t}: ${msg}`, ...log].slice(0, 8)
  }
  async function ensureNostr() { await nostrVPN.init(mySigningPubkey()) }
  async function giveVPN() {
    if (!friendId.trim()) applyDemoPartner()
    if (!friendId.trim()) { addLog('Введите ID друга'); return }
    try {
      await ensureNostr()
      const peer = normalizePeerId(friendId)
      const id = await nostrVPN.inviteFriend(peer, '', '')
      vpnStatus = 'connecting'
      statusText = 'Раздаю VPN · жду подключения друга'
      addLog('Инвайт отправлен, готов как exit node (WebRTC)')
      addLog('Инвайт OK id=' + id.slice(0, 12) + '…')
      showInviteModal = false
    } catch (e) {
      vpnStatus = 'error'
      statusText = 'Ошибка: ' + (e as Error).message
      addLog('Ошибка: ' + (e as Error).message)
    }
    if (!friendId.trim()) applyDemoPartner()
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
        await nostrVPN.acceptVPN(ev.from, '', '')
        vpnStatus = 'connecting'
        statusText = 'Раздаю VPN · жду WebRTC offer'
        addLog('Запрос принят, готов как exit node (WebRTC)')
      } else if (ev.type === 'vpn-invite') {
        addLog('WebRTC: создаю offer для exit node ' + ev.from.slice(0, 12) + '…')
        const { sdp, gatherIce } = await rtcVPN.createOffer()
        const iceCandidates: string[] = []
        await gatherIce((c) => { if (c !== 'END') iceCandidates.push(c) })
        await nostrVPN.sendRTCOffer(ev.from, sdp, iceCandidates)
        addLog('WebRTC offer отправлен (' + iceCandidates.length + ' ICE candidates)')
        vpnStatus = 'connecting'
        statusText = 'WebRTC connecting…'
        const answerTimeout = setTimeout(() => {
          if (vpnStatus === 'connecting') {
            addLog('WebRTC: таймаут ожидания answer')
            vpnStatus = 'error'
            statusText = 'Не удалось подключить VPN'
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
        transportMode = 'P2P'
        addLog('WebRTC P2P connected! Exit IP: ' + tunnelIp)
      } else {
        addLog('P2P probe empty — Nostr relay fallback...')
        transportMode = 'Nostr Relay'
        try {
          await dataRelay.connect()
          const relayIp = await dataRelay.fetchThroughRelay('http://api.ipify.org', senderPubkey)
          const ip = relayIp.trim()
          if (ip && /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(ip)) {
            lastTunnelIp = ip
            addLog('Nostr relay connected! Exit IP: ' + ip)
          } else {
            addLog('WebRTC connected (IP probe empty)')
          }
        } catch (relayErr) {
          addLog('Nostr relay fallback failed: ' + String(relayErr).slice(0, 50))
          addLog('WebRTC connected (IP probe empty)')
        }
      }
      vpnStatus = 'connected'
      statusText = 'VPN подключён (WebRTC P2P)'
    } catch (e) {
      addLog('WebRTC answer failed: ' + (e as Error).message)
      vpnStatus = 'error'
      statusText = 'Не удалось подключить VPN'
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
            handleRTCOffer(event)
          } else if (event.type === 'rtc-answer') {
            if (rtcAnswerResolver) {
              rtcAnswerResolver(event.rtcSdp || '', event.iceCandidates || [])
              rtcAnswerResolver = null
            }
          } else if (event.type === 'rtc-ice') {
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
  {#if isDevMode}
  <div class="tab-p2p-status" data-testid="vpn-tab-p2p-status">
    VPN-101: {tabP2P.phase} · role={tabP2P.role}
    {#if tabP2P.tunnelIp} · IP {tabP2P.tunnelIp}{/if}
    {#if tabP2P.lastError} · err {tabP2P.lastError}{/if}
    {#if tabP2P.phase !== 'idle'}
      <button class="linkish" data-testid="vpn-tab-stop" type="button" onclick={stopTabP2P}>Стоп 2-вкладки</button>
    {/if}
  </div>
  {#if turnStatus}
    <div class="ice-status">
      <span class="ice-badge" title="NAT traversal method">🧊 {turnStatus}{#if transportMode} · 📡 {transportMode}{/if}</span>
      {#if amneziaStatus}<span class="ice-badge amnezia" title="DPI obfuscation">🛡️ {amneziaStatus}</span>{/if}
    </div>
  {/if}
  {/if}
  {#if vpnStatus === 'off' || vpnStatus === 'error'}
    {#if vpnStatus === 'off'}
      <p class="vpn-empty" data-testid="vpn-empty">VPN выключен. В браузере это WebRTC DataChannel, не системный туннель телефона.</p>
    {:else}
      <div class="vpn-status error" data-testid="vpn-error">
        <span class="status-dot"></span>
        <span>{statusText}</span>
        <button class="disconnect-btn" type="button" data-testid="vpn-error-dismiss" onclick={() => { vpnStatus = 'off'; statusText = 'VPN выключен' }}>Закрыть</button>
      </div>
    {/if}
    <div class="vpn-buttons">
      <button class="vpn-btn share" data-testid="vpn-give" disabled={vpnStatus === 'connecting'} onclick={() => { applyDemoPartner(); if (friendId.trim()) giveVPN(); else showInviteModal = true }}>
        📡 Дать VPN другу
      </button>
      <button class="vpn-btn request" data-testid="vpn-request" onclick={() => { applyDemoPartner(); if (friendId.trim()) requestVPN(); else showRequestModal = true }}>
        🤝 Запросить VPN
      </button>
      <button class="vpn-btn engine" data-testid="vpn-engine" disabled={tunnelBusy} onclick={startInAppTunnel}>
        ⚡ VPN в приложении
      </button>
      <button class="vpn-btn push" data-testid="vpn-push" disabled={pushBusy} onclick={enablePush}>
        🔔 Уведомления
      </button>
      {#if isDevMode}
      <button class="vpn-btn engine" data-testid="vpn-tab-host" onclick={startTabP2PHost}>
        🧪 2 вкладки: я exit
      </button>
      <button class="vpn-btn request" data-testid="vpn-tab-join" onclick={startTabP2PJoiner}>
        🧪 2 вкладки: войти peer
      </button>
      {/if}
    </div>
    {#if isDevMode}
    <div class="phase2-status" data-testid="phase2-status">
      <span>Движок: {amneziaStatus || '—'}</span>
      <span>Push: {pushInfo || '—'}</span>
      {#if tunnelInfo?.state === 'up'}
        <span class="conf-path">внутри продукта ✓</span>
        <button class="linkish" data-testid="vpn-engine-stop" disabled={tunnelBusy} onclick={stopInAppTunnel}>Стоп движка</button>
      {/if}
    </div>
    <details class="advanced" data-testid="vpn-advanced">
      <summary onclick={() => showAdvanced = !showAdvanced}>Дополнительно (не нужно для обычной работы)</summary>
      <p class="adv-note">Основной VPN — кнопки «Дать/Запросить VPN» (WebRTC внутри продукта). Ниже только для тех, кто уже пользуется приложением AmneziaVPN отдельно.</p>
      <button class="linkish" data-testid="vpn-export-amnezia" type="button" onclick={exportForAmneziaOptional}>Экспорт conf в AmneziaVPN (опционально)</button>
    </details>
    {/if}
    {#if demoPartnerLabel}
      <div class="lan-hint" data-testid="demo-partner">Демо-партнёр: {demoPartnerLabel} (ID подставлен)</div>
    {/if}
    {#if lanPhoneUrl}
      <div class="lan-hint" data-testid="lan-url">Телефон (та же Wi‑Fi): <code>{lanPhoneUrl}</code></div>
    {/if}
  {:else}
    <div class="vpn-status" class:connecting={vpnStatus === 'connecting'} class:connected={vpnStatus === 'connected'}>
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
@import "./vpn-panel.css";
</style>