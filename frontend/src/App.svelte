<script>
  import { onMount } from 'svelte';
  import { activeTab, toastMessage, toastVisible } from './lib/stores.js';
  import * as OfflineStorage from './lib/offline.js';
  import * as E2E from './lib/e2e.js';
  import { loadIdentity, getWS, refreshOnline } from './lib/ws.js';
  import { getToken, clearToken } from './lib/api.js';
  import Login from './components/Login.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import SkipNav from './components/SkipNav.svelte';
  import Chat from './components/Chat.svelte';
  import VpnPanel from './components/VpnPanel.svelte';
  import ChannelsPanel from './components/ChannelsPanel.svelte';
  import IdentityPanel from './components/IdentityPanel.svelte';
  import ContactsPanel from './components/ContactsPanel.svelte';
  import WebRtcMvp from './components/WebRtcMvp.svelte';
  import { theme } from './lib/stores.js';

  let currentTab = $state('chat');
  let toastMsg = $state('');
  let toastShow = $state(false);
  let isLoggedIn = $state(false);
  let authChecked = $state(false);

  // Sync with store
  activeTab.subscribe((v) => { currentTab = v; });
  toastMessage.subscribe((v) => { toastMsg = v; });
  toastVisible.subscribe((v) => { toastShow = v; });

  function switchTab(name) {
    activeTab.set(name);
    const mainContent = document.getElementById('main-content');
    if (mainContent) {
      mainContent.focus();
    }
  }

  function onLoginSuccess() {
    isLoggedIn = true;
    initApp();
  }

  function handleLogout() {
    clearToken();
    isLoggedIn = false;
  }

  async function initApp() {
    // Initialize offline storage
    try {
      await OfflineStorage.init();
      const cached = await OfflineStorage.getMessageCount();
      if (cached > 0) console.log('Offline cache:', cached, 'messages');
    } catch (e) {
      console.warn('OfflineStorage init failed:', e);
    }

    // Initialize E2E
    try {
      const pubKey = await E2E.init((newPubKey) => {
        const ws = getWS();
        if (ws && ws.readyState === WebSocket.OPEN && newPubKey) {
          ws.send(JSON.stringify({ type: 'key_exchange', publicKey: newPubKey }));
          console.log('E2E key rotated, broadcasting new key');
        }
      });
      console.log('E2E ready, pubkey:', pubKey?.substring(0, 16) + '...');
    } catch (e) {
      console.warn('E2E init failed:', e);
    }

    // PWA
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/sw.js').catch(() => {});
    }

    await loadIdentity();
    setInterval(refreshOnline, 10000);
  }

  onMount(() => {
    // Initial theme set
    theme.subscribe(t => {
      document.body.className = t;
    });

    const token = getToken();
    if (token) {
      isLoggedIn = true;
      authChecked = true;
      initApp();
    } else {
      authChecked = true;
    }
  });
</script>

<SkipNav />

{#if !authChecked}
  <div class="loading-screen">
    <div class="loading-spinner"></div>
    <p>Загрузка...</p>
  </div>
{:else if !isLoggedIn}
  <Login onSuccess={onLoginSuccess} />
{:else}
<div class="app">
  <nav role="navigation" aria-label="Main navigation">
    <Sidebar {currentTab} {switchTab} onLogout={handleLogout} />
  </nav>

  <div class="content" id="main-content" role="main" tabindex="-1" aria-label="{currentTab} panel">
    {#if currentTab === 'chat'}
      <Chat />
    {:else if currentTab === 'vpn'}
      <VpnPanel />
    {:else if currentTab === 'channels'}
      <ChannelsPanel />
    {:else if currentTab === 'identity'}
      <IdentityPanel />
    {:else if currentTab === 'contacts'}
      <ContactsPanel />
    {:else if currentTab === 'mvp'}
      <WebRtcMvp />
    {/if}
  </div>
</div>
{/if}

<div class="toast" class:show={toastShow} role="status" aria-live="polite" aria-atomic="true">{toastMsg}</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }
  
  :global(:root) {
    /* Base configuration */
    --font-family: 'Outfit', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  }

  /* Dark Theme Variables */
  :global(body.dark) {
    --text-primary: #ffffff;
    --text-secondary: rgba(255, 255, 255, 0.7);
    --text-muted: rgba(255, 255, 255, 0.4);
    --bg-body: #050505;
    --bg-app-grad: radial-gradient(circle at top left, #15151f, #050505);
    --bg-panel: rgba(20, 20, 25, 0.6);
    --bg-glass: rgba(255, 255, 255, 0.05);
    --border-glass: rgba(255, 255, 255, 0.08);
    --border-strong: rgba(255, 255, 255, 0.2);
    --accent: #4fc3f7;
    --accent-hover: #81d4fa;
    --accent-active: #29b6f6;
    --error: #ff5252;
    --success: #81c784;
    --shadow-glass: 0 8px 32px 0 rgba(0, 0, 0, 0.5);
    --selection-bg: rgba(30, 136, 229, 0.5);
  }

  /* Light Theme Variables */
  :global(body.light) {
    --text-primary: #000000;
    --text-secondary: rgba(0, 0, 0, 0.65);
    --text-muted: rgba(0, 0, 0, 0.4);
    --bg-body: #f5f7fa;
    --bg-app-grad: radial-gradient(circle at top left, #ffffff, #e4e7eb);
    --bg-panel: rgba(255, 255, 255, 0.8);
    --bg-glass: rgba(0, 0, 0, 0.03);
    --border-glass: rgba(0, 0, 0, 0.06);
    --border-strong: rgba(0, 0, 0, 0.15);
    --accent: #0288d1;
    --accent-hover: #039be5;
    --accent-active: #0277bd;
    --error: #d32f2f;
    --success: #388e3c;
    --shadow-glass: 0 8px 32px 0 rgba(0, 0, 0, 0.05);
    --selection-bg: rgba(30, 136, 229, 0.2);
  }

  :global(body) {
    font-family: var(--font-family);
    background: var(--bg-body);
    color: var(--text-primary);
    height: 100vh;
    overflow: hidden;
    transition: background 0.3s ease, color 0.3s ease;
  }
  :global(::selection) {
    background: var(--selection-bg);
    color: var(--text-primary);
  }
  :global(:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .app {
    display: flex;
    height: 100vh;
    background: var(--bg-app-grad);
  }
  .content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: var(--bg-panel);
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
  }
  .content:focus {
    outline: none;
  }
  .toast {
    position: fixed;
    bottom: 80px;
    left: 50%;
    transform: translateX(-50%);
    padding: 14px 24px;
    background: var(--bg-glass);
    backdrop-filter: blur(12px);
    border: 1px solid var(--border-glass);
    box-shadow: var(--shadow-glass);
    color: var(--text-primary);
    border-radius: 12px;
    font-size: 14px;
    font-weight: 500;
    opacity: 0;
    transition: opacity 0.3s ease-out;
    pointer-events: none;
    z-index: 100;
    white-space: nowrap;
  }
  .toast.show {
    opacity: 1;
  }
  .loading-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    gap: 16px;
    background: var(--bg-body);
  }
  .loading-screen p {
    color: var(--text-primary);
    font-size: 16px;
    font-weight: 500;
  }
  .loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid var(--border-glass);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s cubic-bezier(0.5, 0, 0.5, 1) infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
