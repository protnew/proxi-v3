<script lang="ts">
  import { onMount } from 'svelte'
  import * as stores from './stores/messenger'
  import { initIdentity, initIdentityAsync, connectRelays, sendPresence, getName, onMessage, onPresence, onTyping, getStatus } from './lib/api'
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

    // 6. Complete identity (signup + JWT token) — MUST finish before WS connect
    statusText = 'Авторизация...'
    const realPk = await initIdentityAsync()
    if (realPk) {
      stores.profile.update(p => ({ ...p, pubkey: realPk }))
    }

    // 7. Connect WebSocket with JWT token
    statusText = 'Подключение к серверу...'
    const relayCount = await connectRelays()
    statusText = `Подключено к ${relayCount} relays`
    console.log('[app] Nostr status:', getStatus())

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
