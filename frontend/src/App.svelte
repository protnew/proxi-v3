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
    font-family: -apple-system, 'Segoe UI', sans-serif;
    background: #0a0a0a;
    color: #e0e0e0;
    height: 100vh;
    overflow: hidden;
  }
  :global(:focus-visible) {
    outline: 2px solid #1e88e5;
    outline-offset: 2px;
  }
  .app {
    display: flex;
    height: 100vh;
  }
  .content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .content:focus {
    outline: none;
  }
  .toast {
    position: fixed;
    bottom: 80px;
    left: 50%;
    transform: translateX(-50%);
    padding: 10px 20px;
    background: #1e88e5;
    color: #fff;
    border-radius: 10px;
    font-size: 13px;
    opacity: 0;
    transition: opacity 0.3s;
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
  }
  .loading-screen p {
    color: #555;
    font-size: 14px;
  }
  .loading-spinner {
    width: 32px;
    height: 32px;
    border: 3px solid #222;
    border-top-color: #1e88e5;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
