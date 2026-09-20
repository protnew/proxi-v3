<script lang="ts">
// Deep-link routing: ?view=settings, ?view=newchat, ?view=advanced, ?role=alice|bob
const urlParams = new URLSearchParams(typeof window !== 'undefined' ? window.location.search : '');
const initialView = urlParams.get('view');
// ?role=alice → tester1 (key 111…), ?role=bob → tester2 (key 222…)
const roleParam = urlParams.get('role');
if (roleParam === 'alice' || roleParam === 'bob') {
  localStorage.setItem('proxi_demo_role', roleParam === 'alice' ? 'tester1' : 'tester2');
}

// Mobile view switching: sidebar <-> chat
let mobileChatOpen = $state(false);
function openMobileChat() { mobileChatOpen = true; }
function closeMobileChat() { mobileChatOpen = false; }

// Detect mobile
let isMobile = $state(typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches);
function syncMobile() {
  isMobile = window.matchMedia('(max-width: 768px)').matches
}

  // Dev mode: show raw SOCKS5/WG settings (hidden in product mode)
  const isDevMode = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('dev') === '1';
/* deep-link applied after stores import */

  import { onMount } from 'svelte'
  import * as stores from './stores/messenger'
  import { initIdentity, initIdentityAsync, connectRelays, sendPresence, getName, onMessage, onPresence, onTyping, getStatus } from './lib/api'
  import { sendDM, getSeckey } from './lib/api'
  import { decryptDM, isEncryptedPayload } from './lib/nip-e2e'
  import { startOutboxWatcher } from './lib/offline-outbox'
  import { getIdentity, type Identity } from './lib/identity'
  import { NostrChat, type NostrMessage } from './lib/nostr-chat'
  import { setOnFileComplete } from './lib/peer-manager'
  import { initTheme } from './lib/theme'
  import { initSounds, playIncoming } from './lib/sounds'
  import Sidebar from './components/Sidebar.svelte'
  import ChatView from './components/ChatView.svelte'
  import Settings from './components/Settings.svelte'
  import NewChat from './components/NewChat.svelte'
  import CallOverlay from './components/CallOverlay.svelte'
  import GroupCreate from './components/GroupCreate.svelte'
  import VPNProductPanel from './components/VPNProductPanel.svelte'
  import AuthScreen from './components/AuthScreen.svelte'
  import { initPush } from './lib/push'
  import { checkForUpdate } from './lib/update-check'
  import { startPresence, stopPresence, setActiveChat } from './lib/nostr-chat'
  import VpnPanel from './components/VpnPanel.svelte'
  import DemoPanel from './components/DemoPanel.svelte'
  import type { Message } from './stores/messenger'


  let loading = $state(true)
  let needsOnboarding = $state(false)
  let statusText = $state('Загрузка...')
  import { showSettings, showNewChat, activeChatId } from './stores/messenger'
if (initialView === 'settings' || initialView === 'advanced') {
  stores.showSettings.set(true);
}
if (initialView === 'newchat') {
  stores.showNewChat.update(() => true);
}

  
  let showSettingsView = $derived($showSettings)
  let showNewChatView = $derived($showNewChat)


  async function maybeDecryptText(from: string, text: string): Promise<string> {
    if (!isEncryptedPayload(text)) return text
    const sk = getSeckey()
    if (!sk) return text
    try {
      const pt = await decryptDM(text, sk, from)
      return pt ?? text
    } catch {
      return text
    }
  }

  onMount(async () => {
    const stopOutbox = startOutboxWatcher(async (to, text) => {
      try {
        const resp: any = await sendDM(to, text)
        if (resp?.status && resp.status < 400) return { ok: true, via: resp?.data?.via || 'dm' }
        return { ok: false, error: 'transport down' }
      } catch (e: any) {
        return { ok: false, error: e?.message || 'err' }
      }
    })

    if (typeof window !== 'undefined') window.addEventListener('beforeunload', () => { try { stopOutbox() } catch {} })

    // Safety timeout: force loading=false after 8s
    const safetyTimer = setTimeout(() => {
      if (loading) {
        console.warn('[app] Init timeout — forcing loading=false')
        loading = false
      }
    }, 8000)
    // 1. Load saved data
    stores.loadProfile()
    stores.loadChats()
    stores.loadContacts()
    initTheme()
    const mq = window.matchMedia('(max-width: 768px)')
    syncMobile()
    mq.addEventListener('change', syncMobile)

    // 2. Init identity
    const pk = initIdentity()
    stores.profile.update(p => ({ ...p, pubkey: pk }))

    // 3. Init sounds
    try {
      initSounds()
    } catch (e) {
      console.warn('[app] initSounds failed (likely AudioContext requires user gesture):', e);
    }

    // 4. Set up callbacks BEFORE connecting
    onMessage((msg: Message) => {
      const chatId = msg.to.startsWith('group:') ? msg.to : `dm:${msg.from}`
      const peerName = getName(msg.from)
      stores.ensureDMChat(msg.from, peerName)
      // HIGH audit fix: decrypt NIP-E2E payloads on receive
      void (async () => {
        const text = await maybeDecryptText(msg.from, msg.text || '')
        stores.addMessage(chatId, { ...msg, text, read: false })
        playIncoming()
        if (Notification.permission === 'granted' && document.hidden) {
          try {
            new Notification('Новое сообщение', {
              body: `${peerName}: ${text.slice(0, 60)}`,
              icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"><text y="32" font-size="32">🛡️</text></svg>'
            })
          } catch {}
        }
        updateTitle()
      })()
    })

    onPresence((pk: string, online: boolean) => {
      stores.setContactOnline(pk, online)
    })

    onTyping((pk: string) => {
      const chatId = `dm:${pk}`
      const cs = stores.chats
      let exists = false
      const unsub = cs.subscribe(v => { exists = v.some(c => c.id === chatId) })
      unsub()
      if (exists) stores.setTyping(chatId, pk)
    })

    // 5. Do not prompt notifications on first paint (club 30). User taps «Уведомления».

    // 6. Browser identity (secp256k1 keys in localStorage, no Go server needed)
    statusText = 'Проверка ключей...'
    const demoRole = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_demo_role') : null;
    let browserId: Identity;
    if (demoRole === 'tester1' || demoRole === 'tester2') {
      const demoKey = demoRole === 'tester1' ? '1'.repeat(64) : '2'.repeat(64);
      // P1: derive the REAL x-only pubkey — signed auth requires pubkey↔sk match
      const { getPublicKey } = await import('@noble/secp256k1')
      const skBytes = new Uint8Array(demoKey.match(/.{2}/g)!.map(h => parseInt(h, 16)))
      const demoPub = Array.from(getPublicKey(skBytes, true).slice(1))
        .map(b => b.toString(16).padStart(2, '0')).join('')
      browserId = { privateKey: demoKey, publicKey: demoPub, npub: 'npub1demo' + demoPub.slice(0, 16), nsec: 'nsec1demo', createdAt: Date.now() };
      console.log('[app] Demo mode identity:', demoRole);
    } else {
      const { loadIdentityAsync, getIdentity, loadStoredPubkey } = await import('./lib/identity')
      const existing = await loadIdentityAsync()
      const storedPub = loadStoredPubkey()
      if (!existing && !storedPub) {
        // ONB-000: first-run product path — do not silent-create keys
        needsOnboarding = true
        loading = false
        return
      }
      browserId = existing || await getIdentity()
    }
    stores.profile.update(p => ({ ...p, pubkey: browserId.publicKey }))
    // Club 30: show chat immediately. Transports continue in background.
    loading = false

    // 7. DUAL TRANSPORT (audit 2026-08-05):
    //    - Nostr = optional serverless path
    //    - Go /ws = ALWAYS when local API healthy (do NOT skip if Nostr connects)
    statusText = 'Подключение транспортов...'
    let nostrChat: NostrChat | null = new NostrChat(browserId)
    setActiveChat(nostrChat)
    nostrChat.onMessage((msg: NostrMessage) => {
      console.log('[app] Nostr DM from', msg.from.slice(0, 16), ':', msg.text.slice(0, 50))
      const peerName = getName(msg.from)
      stores.ensureDMChat(msg.from, peerName)
      void (async () => {
        const text = await maybeDecryptText(msg.from, msg.text || '')
        stores.addMessage(`dm:${msg.from}`, {
          id: msg.id, from: msg.from, to: msg.to, text,
          timestamp: msg.timestamp, type: 'text', read: false,
        })
        playIncoming()
      })()
    })
    // Go hub first — signup JWT lands in localStorage, then Nostr
    // connects to /nostr with ?token= (BAG-30: was connecting before
    // the token existed → guaranteed 401 on first attempt).
    let goRelays = 0
    try {
      // Probe health first (same-origin or VITE_API_URL)
      const healthUrl = ((import.meta as any).env?.VITE_API_URL || '') + '/api/health'
      let healthOk = false
      try {
        const hr = await fetch(healthUrl || '/api/health', { method: 'GET' })
        healthOk = hr.ok
      } catch {
        healthOk = false
      }
      if (healthOk) {
        const realPk = await initIdentityAsync()
        if (realPk) stores.profile.update(p => ({ ...p, pubkey: realPk }))
        goRelays = await connectRelays()
      } else {
        console.warn('[app] Go /api/health not OK — skip local WS')
      }
    } catch (e) {
      console.warn('[app] Go transport failed:', e)
      goRelays = 0
    }

    let nostrRelays = 0
    try {
      // Timeout relay connection — don't let it block the UI
      nostrRelays = await Promise.race([
        nostrChat.connect(),
        new Promise<number>((resolve) => setTimeout(() => resolve(0), 10000)),
      ])
    } catch (e) {
      console.warn('[app] Nostr connect failed:', e)
      nostrRelays = 0
    }
    if (nostrRelays === 0) { nostrChat = null; setActiveChat(null) }

    const parts: string[] = []
    if (goRelays > 0) parts.push(`Go WS: ${goRelays}`)
    if (nostrRelays > 0) parts.push(`Nostr: ${nostrRelays}`)
    statusText = parts.length ? parts.join(' + ') : 'Offline mode (messages queued)'
    console.log('[app] Transport dual:', { goRelays, nostrRelays, statusText })

    // Expose transports for ChatView / DemoPanel
    ;(window as any).__nostrChat = nostrChat
    ;(window as any).__goRelays = goRelays
    ;(window as any).__nostrRelays = nostrRelays
    ;(window as any).__transportStatus = {
      go: goRelays > 0,
      nostr: nostrRelays > 0,
      label: statusText,
    }

    // Set up incoming file handler
    setOnFileComplete((blob: Blob, manifest) => {
      const url = URL.createObjectURL(blob)
      const msg: Message = {
        id: manifest.id,
        from: manifest.from,
        to: '',
        text: `📎 ${manifest.name}`,
        timestamp: Date.now(),
        type: 'file',
        fileName: manifest.name,
        fileSize: manifest.size,
        fileUrl: url,
        read: false,
      }
      const chatId = `dm:${manifest.from}`
      stores.ensureDMChat(manifest.from, getName(manifest.from))
      stores.addMessage(chatId, msg)
      playIncoming()
    })

    sendPresence(true)

    // Auto-setup demo contacts: Alice and Bob pre-linked
    // Real x-only pubkeys of demo secrets '1'*64 / '2'*64 (P1 signed auth)
    const DEMO_ALICE = '4f355bdcb7cc0af728ef3cceb9615d90684bb5b2ca5f859ab0f0b704075871aa';
    const DEMO_BOB = '466d7fcae563e5cb09a0d1870bb580344804617879a14949cf22285f1bae3f27';
    const myPk = pk || browserId.publicKey; // prefer demo key from initIdentity
    if (myPk === DEMO_ALICE || myPk === DEMO_BOB) {
      const isAlice = myPk === DEMO_ALICE;
      const partnerPk = isAlice ? DEMO_BOB : DEMO_ALICE;
      const partnerName = isAlice ? 'Bob' : 'Alice';
      // Pre-create DM chat with partner
      stores.ensureDMChat(partnerPk, partnerName);
      stores.setContactOnline(partnerPk, true);
      // Auto-select the chat
      stores.activeChatId.set(`dm:${partnerPk}`);
      console.log(`[app] Demo mode: ${isAlice ? 'Alice' : 'Bob'} → chat with ${partnerName} ready`);
    }

    clearTimeout(safetyTimer)

    // INF-003: Check for PWA updates + listen for update event
    window.addEventListener('pwa-update-available', () => { updateAvailable = true })
    checkForUpdate().catch(() => {})

    // PUSH-001/003: Init push notifications
    initPush().catch(() => {})

    // MSG-105: Start presence broadcasting
    try { startPresence(browserId) } catch {}

    loading = false
  })

  function updateTitle() {
    const unsub = stores.chats.subscribe(cs => {
      const unread = cs.reduce((sum, c) => sum + c.unread, 0)
      document.title = unread > 0 ? `(${unread}) Proxi` : 'Proxi'
    })
    unsub()
  }

  window.addEventListener('beforeunload', () => {
    stores.saveChats()
    stores.saveContacts()
    stores.saveProfile()
    sendPresence(false)
  })

  setInterval(() => { stores.saveChats(); stores.saveContacts() }, 30000)
</script>

{#if needsOnboarding}
    <AuthScreen onDone={() => { needsOnboarding = false; location.reload() }} />
  {:else}
  <div class="app">
    {#if loading}
    <div class="loading-overlay-bg" id="loading-overlay">
      <div class="skeleton-col" aria-hidden="true">
        <div class="skeleton-chat"></div>
        <div class="skeleton-chat"></div>
        <div class="skeleton-chat"></div>
      </div>
      <div class="loading-status">
        <div class="spinner"></div>
        <p>{statusText}</p>
      </div>
    </div>
    {/if}
    {#if showSettingsView}
      <div class="settings-mobile-wrapper" class:hidden-mobile={mobileChatOpen}>
        <Settings />
      </div>
    {:else}
      <div class="sidebar-mobile-wrapper" class:hidden-mobile={mobileChatOpen}>
        <div class="sidebar-stack">
          <Sidebar on:chatselect={openMobileChat} />
          {#if !isDevMode}
            <VPNProductPanel />
          {/if}
        </div>
      </div>
    {/if}
    <div class="chat-mobile-wrapper" class:hidden-mobile={!mobileChatOpen && isMobile}>
      <ChatView on:back={closeMobileChat} />
    </div>
    <NewChat />
    <GroupCreate />
    <CallOverlay />
    {#if isDevMode}
      <VpnPanel />
      <DemoPanel />
    {/if}
  </div>
  {/if}

<style>
  :global(body) { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; transition: background 180ms ease-in-out, color 180ms ease-in-out; }
  :global(*) { box-sizing: border-box; }

  /* Dark theme — HEX only here */
  :global(:root), :global(:root[data-theme="dark"]) {
    --bg: #0f172a;
    --bg-secondary: rgba(30, 41, 59, 0.7);
    --bg-tertiary: rgba(51, 65, 85, 0.6);
    --bg-hover: rgba(71, 85, 105, 0.5);
    --bg-active: rgba(56, 189, 248, 0.2);
    --text: #f8fafc;
    --text-muted: #94a3b8;
    --text-dim: #94a3b8;
    --border: #475569;
    --accent: #1d4ed8;
    --accent-light: #60a5fa;
    --text-on-accent: #f8fafc;
    --success: #10b981;
    --danger: #ef4444;
    --warn: #fbbf24;
    --shadow: 0 1px 3px rgba(0, 0, 0, 0.24);
    --bubble: rgba(30, 41, 59, 0.8);
    --bubble-mine: linear-gradient(135deg, #1d4ed8, #1e40af);
  }

  :global(:root[data-theme="light"]) {
    --bg: #f8fafc;
    --bg-secondary: rgba(255, 255, 255, 0.92);
    --bg-tertiary: rgba(241, 245, 249, 0.9);
    --bg-hover: rgba(226, 232, 240, 0.8);
    --bg-active: rgba(29, 78, 216, 0.12);
    --text: #0f172a;
    --text-muted: #374151;
    --text-dim: #374151;
    --border: #64748b;
    --accent: #1e40af;
    --accent-light: #2563eb;
    --text-on-accent: #ffffff;
    --success: #059669;
    --danger: #dc2626;
    --warn: #b45309;
    --shadow: 0 1px 3px rgba(15, 23, 42, 0.12);
    --bubble: rgba(255, 255, 255, 0.9);
    --bubble-mine: linear-gradient(135deg, #1e40af, #1d4ed8);
  }
  :global(::-webkit-scrollbar) { width: 8px; }
  :global(::-webkit-scrollbar-track) { background: transparent; }
  :global(::-webkit-scrollbar-thumb) { background: var(--border); border-radius: 8px; }
  :global(button) { font-family: inherit; }

  .app { display: flex; height: 100vh; overflow: hidden; position: relative; }

  .loading-overlay-bg {
    position: absolute; inset: 0; z-index: 5;
    display: flex; align-items: stretch;
    pointer-events: none;
    background: color-mix(in srgb, var(--bg) 72%, transparent);
  }
  .skeleton-col { width: 280px; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
  .skeleton-chat {
    min-height: 64px; border-radius: 12px;
    background: linear-gradient(90deg, var(--bg-secondary) 0%, var(--bg-hover) 50%, var(--bg-secondary) 100%);
    background-size: 200% 100%;
  }
  .loading-status {
    flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  }
  .spinner { width: 40px; height: 40px; border: 3px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin 0.8s linear infinite; margin-bottom: 16px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .loading-overlay-bg p { color: var(--text-muted); font-size: 16px; }

  /* Mobile wrappers */
  .sidebar-mobile-wrapper { height: 100%; }
  .sidebar-stack {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 360px;
    min-width: 360px;
    border-right: 1px solid var(--border);
  }
  .sidebar-stack :global(.sidebar) {
    flex: 1;
    min-height: 0;
    height: auto;
    width: 100%;
    min-width: 0;
    border-right: none;
  }
  .sidebar-stack :global(.vpn-product) {
    flex-shrink: 0;
    border-top: 1px solid var(--border);
  }
  .chat-mobile-wrapper { flex: 1; height: 100%; min-width: 0; }
  .settings-mobile-wrapper { width: 100%; height: 100%; overflow-y: auto; }
  @media (max-width: 768px) {
    .sidebar-mobile-wrapper { width: 100%; }
    .sidebar-stack { width: 100%; min-width: 100%; }
    .sidebar-mobile-wrapper.hidden-mobile { display: none; }
    .chat-mobile-wrapper.hidden-mobile { display: none; }
    .settings-mobile-wrapper.hidden-mobile { display: none; }
    .chat-mobile-wrapper { width: 100%; flex: none; }
  }

  @media (prefers-reduced-motion: reduce) {
    :global(*), :global(*::before), :global(*::after) {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
    }
  }
</style>
