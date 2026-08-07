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
const isMobile = typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches;

  // Dev mode: show raw SOCKS5/WG settings (hidden in product mode)
  const isDevMode = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('dev') === '1';
if (initialView === 'settings') {
  setTimeout(() => { stores.showSettings.set(true); }, 500);
}
if (initialView === 'newchat') {
  setTimeout(() => { stores.showNewChat.update(() => true); }, 500);
}
if (initialView === 'advanced') {
  setTimeout(() => {
    stores.showSettings.set(true);
    setTimeout(() => {
      const gearBtn = document.querySelector('button:last-child');
      if (gearBtn) (gearBtn as HTMLElement).click();
    }, 300);
  }, 500);
}

  import { onMount } from 'svelte'
  import * as stores from './stores/messenger'
  import { initIdentity, initIdentityAsync, connectRelays, sendPresence, getName, onMessage, onPresence, onTyping, getStatus } from './lib/api'
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
  import DemoPanel from './components/DemoPanel.svelte'
  import type { Message } from './stores/messenger'

  console.log('[app] App.svelte script execution started');

  let loading = $state(true)
  let statusText = $state('Загрузка...')
  import { showSettings, showNewChat, activeChatId } from './stores/messenger'
  
  let showSettingsView = $derived($showSettings)
  let showNewChatView = $derived($showNewChat)

  onMount(async () => {
    // Safety timeout: force loading=false after 8s
    const safetyTimer = setTimeout(() => {
      if (loading) {
        console.warn('[app] Init timeout — forcing loading=false')
        loading = false
      }
    }, 8000)
    console.log('[app] onMount started!');
    // 1. Load saved data
    stores.loadProfile()
    stores.loadChats()
    stores.loadContacts()
    initTheme()

    // 2. Init identity
    const pk = initIdentity()
    stores.profile.update(p => ({ ...p, pubkey: pk }))

    // 3. Init sounds
    console.log('[app] Before initSounds');
    try {
      initSounds()
    } catch (e) {
      console.warn('[app] initSounds failed (likely AudioContext requires user gesture):', e);
    }
    console.log('[app] After initSounds');

    // 4. Set up callbacks BEFORE connecting
    onMessage((msg: Message) => {
      const chatId = msg.to.startsWith('group:') ? msg.to : `dm:${msg.from}`
      const peerName = getName(msg.from)
      stores.ensureDMChat(msg.from, peerName)
      stores.addMessage(chatId, { ...msg, read: false })
      playIncoming()

      if (Notification.permission === 'granted' && document.hidden) {
        try {
          new Notification('Новое сообщение', {
            body: `${peerName}: ${msg.text.slice(0, 60)}`,
            icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"><text y="32" font-size="32">🛡️</text></svg>'
          })
        } catch {}
      }
      updateTitle()
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

    // 5. Request notification permission
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission()
    }

    // 6. Browser identity (secp256k1 keys in localStorage, no Go server needed)
    statusText = 'Генерация ключей...'
    // Demo mode: if initIdentity set demo keys, preserve them
    const demoRole = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_demo_role') : null;
    let browserId: Identity;
    if (demoRole === 'tester1' || demoRole === 'tester2') {
      const demoKey = demoRole === 'tester1' ? '1'.repeat(64) : '2'.repeat(64);
      browserId = { privateKey: demoKey, publicKey: demoKey, npub: 'npub1demo' + demoKey.slice(0, 16) };
      console.log('[app] Demo mode identity:', demoRole);
    } else {
      browserId = await getIdentity();
    }
    stores.profile.update(p => ({ ...p, pubkey: browserId.publicKey }))
    console.log('[app] Identity:', browserId.publicKey.slice(0, 16) + '...')

    // 7. DUAL TRANSPORT (audit 2026-08-05):
    //    - Nostr = optional serverless path
    //    - Go /ws = ALWAYS when local API healthy (do NOT skip if Nostr connects)
    statusText = 'Подключение транспортов...'
    let nostrChat: NostrChat | null = new NostrChat(browserId)
    nostrChat.onMessage((msg: NostrMessage) => {
      console.log('[app] Nostr DM from', msg.from.slice(0, 16), ':', msg.text.slice(0, 50))
      const peerName = getName(msg.from)
      stores.ensureDMChat(msg.from, peerName)
      stores.addMessage(`dm:${msg.from}`, {
        id: msg.id, from: msg.from, to: msg.to, text: msg.text,
        timestamp: msg.timestamp, type: 'text', read: false,
      })
      playIncoming()
    })
    let nostrRelays = 0
    try {
      // Timeout external relay connections — don't let them block the UI
      nostrRelays = await Promise.race([
        nostrChat.connect(),
        new Promise<number>((resolve) => setTimeout(() => resolve(0), 4000)),
      ])
    } catch (e) {
      console.warn('[app] Nostr connect failed:', e)
      nostrRelays = 0
    }
    if (nostrRelays === 0) nostrChat = null

    // Go hub — always attempt (signup JWT + /ws). Independent of Nostr.
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
    const DEMO_ALICE = '1'.repeat(64);
    const DEMO_BOB = '2'.repeat(64);
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
    loading = false
  })

  function updateTitle() {
    const unsub = stores.chats.subscribe(cs => {
      const unread = cs.reduce((sum, c) => sum + c.unread, 0)
      document.title = unread > 0 ? `(${unread}) Indestructible Messenger` : 'Indestructible Messenger'
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

{#if loading}
  <div class="loading" id="loading-overlay">
    <div class="spinner"></div>
    <p>🛡️ {statusText}</p>
  </div>
{:else}
  <div class="app">
    {#if showSettingsView}
      <div class="settings-mobile-wrapper" class:hidden-mobile={mobileChatOpen}>
        <Settings />
      </div>
    {:else}
      <div class="sidebar-mobile-wrapper" class:hidden-mobile={mobileChatOpen}>
        <Sidebar on:chatselect={openMobileChat} />
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
    {:else}
      <VPNProductPanel />
    {/if}
    <DemoPanel />
  </div>
{/if}

<style>
  :global(body) { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; transition: background 0.3s, color 0.3s; }
  :global(*) { box-sizing: border-box; }

  /* Dark theme (Premium Glassmorphism) */
  :global(:root), :global(:root[data-theme="dark"]) {
    --bg: #0f172a;
    --bg-secondary: rgba(30, 41, 59, 0.7);
    --bg-tertiary: rgba(51, 65, 85, 0.6);
    --bg-hover: rgba(71, 85, 105, 0.5);
    --bg-active: rgba(56, 189, 248, 0.2);
    --text: #f8fafc;
    --text-muted: #94a3b8;
    --text-dim: #64748b;
    --border: rgba(255, 255, 255, 0.1);
    --accent: #3b82f6;
    --accent-light: #60a5fa;
    --success: #10b981;
    --danger: #ef4444;
    --bubble: rgba(30, 41, 59, 0.8);
    --bubble-mine: linear-gradient(135deg, #3b82f6, #6366f1);
  }

  /* Light theme */
  :global(:root[data-theme="light"]) {
    --bg: #f8fafc;
    --bg-secondary: rgba(255, 255, 255, 0.8);
    --bg-tertiary: rgba(241, 245, 249, 0.7);
    --bg-hover: rgba(226, 232, 240, 0.6);
    --bg-active: rgba(59, 130, 246, 0.1);
    --text: #0f172a;
    --text-muted: #64748b;
    --text-dim: #94a3b8;
    --border: rgba(0, 0, 0, 0.05);
    --accent: #2563eb;
    --accent-light: #3b82f6;
    --success: #059669;
    --danger: #dc2626;
    --bubble: rgba(255, 255, 255, 0.9);
    --bubble-mine: linear-gradient(135deg, #3b82f6, #4f46e5);
  }
  :global(::-webkit-scrollbar) { width: 6px; }
  :global(::-webkit-scrollbar-track) { background: transparent; }
  :global(::-webkit-scrollbar-thumb) { background: #333; border-radius: 3px; }
  :global(button) { font-family: inherit; }

  .app { display: flex; height: 100vh; overflow: hidden; }

  .loading { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; }
  .spinner { width: 40px; height: 40px; border: 3px solid #2a2a4a; border-top-color: #3a7bd5; border-radius: 50%; animation: spin 0.8s linear infinite; margin-bottom: 16px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .loading p { color: #7a8a9a; font-size: 14px; }

  /* Mobile wrappers */
  .sidebar-mobile-wrapper { height: 100%; }
  .chat-mobile-wrapper { flex: 1; height: 100%; min-width: 0; }
  .settings-mobile-wrapper { width: 100%; height: 100%; overflow-y: auto; }
  @media (max-width: 768px) {
    .sidebar-mobile-wrapper { width: 100%; }
    .sidebar-mobile-wrapper.hidden-mobile { display: none; }
    .chat-mobile-wrapper.hidden-mobile { display: none; }
    .settings-mobile-wrapper.hidden-mobile { display: none; }
    .chat-mobile-wrapper { width: 100%; flex: none; }
  }

</style>
