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
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    background: #050505;
    color: #ffffff;
    height: 100vh;
    overflow: hidden;
  }
  :global(::selection) {
    background: rgba(30, 136, 229, 0.5);
    color: #ffffff;
  }
  :global(:focus-visible) {
    outline: 2px solid #ffffff;
    outline-offset: 2px;
  }
  .app {
    display: flex;
    height: 100vh;
    background: radial-gradient(circle at top left, #111111, #000000);
  }
  .content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: rgba(10, 10, 10, 0.7);
    backdrop-filter: blur(20px);
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
    background: rgba(255, 255, 255, 0.15);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.2);
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
    color: #ffffff;
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
    background: #000000;
  }
  .loading-screen p {
    color: #ffffff;
    font-size: 16px;
    font-weight: 500;
  }
  .loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255, 255, 255, 0.1);
    border-top-color: #ffffff;
    border-radius: 50%;
    animation: spin 0.8s cubic-bezier(0.5, 0, 0.5, 1) infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
