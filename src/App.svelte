<script>
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import VpnPanel from './components/VpnPanel.svelte';

  let activeTab = $state('chat');

  onMount(async () => {
    // Check VPN status on load
  });
</script>

<main class="app">
  <Sidebar bind:activeTab />
  <div class="content">
    {#if activeTab === 'chat'}
      <ChatPanel />
    {:else if activeTab === 'vpn'}
      <VpnPanel />
    {:else if activeTab === 'channels'}
      <div class="placeholder"><h2>Каналы — Phase 2</h2></div>
    {/if}
  </div>
</main>

<style>
  :global(*) { margin: 0; padding: 0; box-sizing: border-box; }
  :global(body) { font-family: 'Inter', -apple-system, sans-serif; background: #0a0a0a; color: #e0e0e0; }
  .app { display: flex; height: 100vh; }
  .content { flex: 1; overflow: hidden; }
  .placeholder { display: flex; align-items: center; justify-content: center; height: 100%; opacity: 0.4; }
</style>
