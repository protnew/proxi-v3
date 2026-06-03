<script>
  import { onMount } from 'svelte';
  import { activeTab, toastMessage, toastVisible } from './lib/stores.js';
  import * as OfflineStorage from './lib/offline.js';
  import * as E2E from './lib/e2e.js';
  import { loadIdentity, getWS, refreshOnline } from './lib/api.js';
  import Sidebar from './components/Sidebar.svelte';
  import SkipNav from './components/SkipNav.svelte';
  import Chat from './components/Chat.svelte';
  import VpnPanel from './components/VpnPanel.svelte';
  import ChannelsPanel from './components/ChannelsPanel.svelte';
  import IdentityPanel from './components/IdentityPanel.svelte';

  let currentTab = $state('chat');
  let toastMsg = $state('');
  let toastShow = $state(false);

  // Sync with store
  activeTab.subscribe((v) => { currentTab = v; });
  toastMessage.subscribe((v) => { toastMsg = v; });
  toastVisible.subscribe((v) => { toastShow = v; });

  function switchTab(name) {
    activeTab.set(name);
    // Focus management: move focus to main content area after tab switch
    const mainContent = document.getElementById('main-content');
    if (mainContent) {
      mainContent.focus();
    }
  }

  onMount(async () => {
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
  });
</script>

<SkipNav />

<div class="app">
  <nav role="navigation" aria-label="Main navigation">
    <Sidebar {currentTab} {switchTab} />
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
</style>
