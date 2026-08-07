<script lang="ts">
  import { nostrVPN, type VPNEvent } from '../lib/nostr-vpn'
  import { wtVPN } from '../lib/webtransport'

  let showInviteModal = $state(false)
  let showRequestModal = $state(false)
  let incomingEvent = $state<VPNEvent | null>(null)
  let friendNpub = $state('')
  let vpnStatus = $state<'off' | 'sharing' | 'connected'>('off')
  let statusText = $state('VPN выключен')
  let log = $state<string[]>([])

  function addLog(msg: string) {
    const t = new Date().toLocaleTimeString()
    log = [`${t}: ${msg}`, ...log].slice(0, 5)
  }

  // "Give VPN to friend" — Alice shares her exit node
  async function giveVPN() {
    if (!friendNpub.trim()) {
      addLog('Введите ID друга')
      return
    }
    try {
      // Start WebTransport server locally (via Go API)
      const token = localStorage.getItem('proxi_token')
      const resp = await fetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify({ method: 'start_real_tunnel', params: {} }),
      })
      const data = await resp.json()
      const wtAddr = data.result?.socksAddr || window.location.hostname + ':10808'

      // Get cert hash from Go WT server
      const resp2 = await fetch('/api/vpn/wt/stats', {
        headers: { Authorization: 'Bearer ' + token },
      })
      let certHash = ''
      if (resp2.ok) {
        const wtStats = await resp2.json()
        certHash = wtStats.certHash || ''
      }

      // Send invite via Nostr
      await nostrVPN.inviteFriend(friendNpub.trim(), wtAddr, certHash)
      vpnStatus = 'sharing'
      statusText = 'Раздаю VPN другу'
      addLog(`Инвайт отправлен другу ${friendNpub.slice(0, 16)}...`)
      showInviteModal = false
    } catch (e) {
      addLog('Ошибка: ' + (e as Error).message)
    }
  }

  // "Request VPN from friend"
  async function requestVPN() {
    if (!friendNpub.trim()) {
      addLog('Введите ID друга')
      return
    }
    try {
      await nostrVPN.requestVPN(friendNpub.trim())
      addLog('Запрос отправлен другу')
      showRequestModal = false
    } catch (e) {
      addLog('Ошибка: ' + (e as Error).message)
    }
  }

  // Accept incoming VPN invite/request
  async function acceptVPN() {
    if (!incomingEvent) return
    try {
      await nostrVPN.acceptVPN(incomingEvent.from, incomingEvent.wtAddr, incomingEvent.wtCertHash)

      // Connect to friend's WebTransport exit node
      if (incomingEvent.wtAddr) {
        await wtVPN.connect(incomingEvent.wtAddr, incomingEvent.wtCertHash)
        vpnStatus = 'connected'
        statusText = 'Подключён к VPN друга'
        addLog('Подключён через WebTransport')
      }
      incomingEvent = null
    } catch (e) {
      addLog('Ошибка подключения: ' + (e as Error).message)
    }
  }

  // Reject incoming VPN
  async function rejectVPN() {
    if (!incomingEvent) return
    await nostrVPN.rejectVPN(incomingEvent.from)
    addLog('Отклонено')
    incomingEvent = null
  }

  // Disconnect
  async function disconnectVPN() {
    await wtVPN.disconnect()
    vpnStatus = 'off'
    statusText = 'VPN выключен'
    addLog('Отключён')
  }

  // Listen for incoming VPN events
  let unsubVPN: (() => void) | null = null
  $effect(() => {
    const token = localStorage.getItem('proxi_token')
    if (!token) return

    // Init Nostr signaling
    const myNpub = localStorage.getItem('proxi_npub') || 'unknown'
    nostrVPN.init(myNpub).then(() => {
      nostrVPN.subscribeToVPNEvents()
      unsubVPN = nostrVPN.onVPNEvent((event) => {
        if (event.type === 'vpn-invite' || event.type === 'vpn-request') {
          incomingEvent = event
          addLog(`Получен ${event.type === 'vpn-invite' ? 'инвайт' : 'запрос'} от ${event.from.slice(0, 16)}...`)
        }
      })
    })

    return () => {
      if (unsubVPN) unsubVPN()
    }
  })
</script>

<div class="vpn-product">
  {#if vpnStatus === 'off'}
    <div class="vpn-buttons">
      <button class="vpn-btn share" onclick={() => showInviteModal = true}>
        📡 Дать VPN другу
      </button>
      <button class="vpn-btn request" onclick={() => showRequestModal = true}>
        🤝 Запросить VPN
      </button>
    </div>
  {:else}
    <div class="vpn-status" class:sharing={vpnStatus === 'sharing'} class:connected={vpnStatus === 'connected'}>
      <span class="status-dot"></span>
      <span>{statusText}</span>
      <button class="disconnect-btn" onclick={disconnectVPN}>Отключить</button>
    </div>
  {/if}

  {#if log.length > 0}
    <div class="vpn-log">
      {#each log as entry}
        <div class="log-entry">{entry}</div>
      {/each}
    </div>
  {/if}
</div>

{#if showInviteModal}
  <div class="modal-overlay" onclick={() => showInviteModal = false}>
    <div class="modal" onclick={(e) => e.stopPropagation()}>
      <h3>📡 Поделиться VPN с другом</h3>
      <p>Введите ID друга (npub), которому хотите раздать интернет:</p>
      <input type="text" placeholder="npub друга..." bind:value={friendNpub} />
      <div class="modal-buttons">
        <button class="cancel" onclick={() => showInviteModal = false}>Отмена</button>
        <button class="confirm" onclick={giveVPN}>Раздать</button>
      </div>
    </div>
  </div>
{/if}

{#if showRequestModal}
  <div class="modal-overlay" onclick={() => showRequestModal = false}>
    <div class="modal" onclick={(e) => e.stopPropagation()}>
      <h3>🤝 Запросить VPN у друга</h3>
      <p>Введите ID друга (npub), у которого хотите запросить доступ:</p>
      <input type="text" placeholder="npub друга..." bind:value={friendNpub} />
      <div class="modal-buttons">
        <button class="cancel" onclick={() => showRequestModal = false}>Отмена</button>
        <button class="confirm" onclick={requestVPN}>Запросить</button>
      </div>
    </div>
  </div>
{/if}

{#if incomingEvent}
  <div class="modal-overlay">
    <div class="modal">
      <h3>{incomingEvent.type === 'vpn-invite' ? '📡 Друг предлагает VPN!' : '🤝 Друг просит VPN'}</h3>
      <p>От: <code>{incomingEvent.from.slice(0, 24)}...</code></p>
      {#if incomingEvent.type === 'vpn-invite'}
        <p>Нажмите «Принять» чтобы подключиться через WebTransport.</p>
      {:else}
        <p>Нажмите «Разрешить» чтобы начать раздавать интернет.</p>
      {/if}
      <div class="modal-buttons">
        <button class="cancel" onclick={rejectVPN}>Отклонить</button>
        <button class="confirm" onclick={acceptVPN}>{incomingEvent.type === 'vpn-invite' ? 'Принять' : 'Разрешить'}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .vpn-product { padding: 12px; }

  .vpn-buttons { display: flex; flex-direction: column; gap: 8px; }
  .vpn-btn { padding: 14px; border: none; border-radius: 12px; font-size: 15px; font-weight: 600; cursor: pointer; font-family: inherit; transition: transform 0.1s; }
  .vpn-btn:active { transform: scale(0.98); }
  .vpn-btn.share { background: linear-gradient(135deg, #1e3a5f, #2a6a4a); color: #fff; }
  .vpn-btn.request { background: linear-gradient(135deg, #2a4a6a, #3b5998); color: #fff; }

  .vpn-status { display: flex; align-items: center; gap: 8px; padding: 14px; border-radius: 12px; font-size: 14px; font-weight: 600; }
  .vpn-status.sharing { background: #0d2818; color: #6ee7b7; border: 1px solid #1a4a2a; }
  .vpn-status.connected { background: #0d1a28; color: #7dd3fc; border: 1px solid #1a3a5a; }
  .status-dot { width: 10px; height: 10px; border-radius: 50%; background: currentColor; animation: pulse 2s infinite; }
  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }
  .disconnect-btn { margin-left: auto; background: rgba(255,107,107,0.2); border: 1px solid #ff6b6b33; color: #ff6b6b; padding: 4px 12px; border-radius: 6px; cursor: pointer; font-size: 12px; font-family: inherit; }

  .vpn-log { margin-top: 8px; max-height: 80px; overflow-y: auto; }
  .log-entry { font-size: 11px; color: #6a7a8a; padding: 2px 0; font-family: monospace; }

  .modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.7); display: flex; align-items: center; justify-content: center; z-index: 1000; padding: 20px; }
  .modal { background: #161b22; border: 1px solid #30363d; border-radius: 16px; padding: 24px; max-width: 400px; width: 100%; }
  .modal h3 { margin: 0 0 8px; font-size: 18px; color: #e0e0e0; }
  .modal p { font-size: 13px; color: #8b949e; margin: 8px 0; }
  .modal input { width: 100%; padding: 10px; background: #0d1117; border: 1px solid #30363d; border-radius: 8px; color: #e0e0e0; font-size: 14px; font-family: monospace; box-sizing: border-box; margin: 8px 0; }
  .modal-buttons { display: flex; gap: 8px; margin-top: 16px; }
  .modal-buttons button { flex: 1; padding: 10px; border: none; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; font-family: inherit; }
  .modal-buttons .cancel { background: #21262d; color: #8b949e; }
  .modal-buttons .confirm { background: #238636; color: #fff; }

  @media (max-width: 768px) {
    .vpn-btn { padding: 16px; font-size: 16px; }
  }
</style>
