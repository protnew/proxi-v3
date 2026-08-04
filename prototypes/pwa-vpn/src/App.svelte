<script lang="ts">
// Deep-link routing: ?view=settings, ?view=newchat, ?view=advanced
const urlParams = new URLSearchParams(typeof window !== 'undefined' ? window.location.search : '');
const initialView = urlParams.get('view');
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
  import DemoPanel from './components/DemoPanel.svelte'
  import type { Message } from './stores/messenger'

  console.log('[app] App.svelte script execution started');

  let loading = $state(true)
  let statusText = $state('Загрузка...')
  import { showSettings, showNewChat, activeChatId } from './stores/messenger'
  
  let showSettingsView = $derived($showSettings)
  let showNewChatView = $derived($showNewChat)

  onMount(async () => {
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

    // 7. Try Nostr relays first (serverless mode)
    statusText = 'Подключение к Nostr relays...'
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
    const nostrRelays = await nostrChat.connect()
    statusText = nostrRelays > 0
      ? `Nostr: ${nostrRelays} relays (serverless)`
      : 'Fallback: Go server mode...'
    console.log('[app] Transport:', nostrRelays > 0 ? 'Nostr (serverless)' : 'Go fallback')

    // SL-014: If Nostr failed, fall back to Go WS (backward compat)
    let goRelays = 0
    if (nostrRelays === 0) {
      nostrChat = null
      try {
        const realPk = await initIdentityAsync()
        if (realPk) stores.profile.update(p => ({ ...p, pubkey: realPk }))
        goRelays = await connectRelays()
        statusText = `Go server: ${goRelays} relay`
      } catch (e) {
        console.warn('[app] Go fallback also failed:', e)
        statusText = 'Offline mode (messages queued)'
      }
    }
    // Expose nostrChat globally for ChatView to use
    ;(window as any).__nostrChat = nostrChat

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
  <div class="loading">
    <div class="spinner"></div>
    <p>🛡️ {statusText}</p>
  </div>
{:else}
  <div class="app">
    {#if showSettingsView}
      <Settings />
    {:else}
      <Sidebar />
    {/if}
    <ChatView />
    <NewChat />
    <GroupCreate />
    <CallOverlay />
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
</style>
