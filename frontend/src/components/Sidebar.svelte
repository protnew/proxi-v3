<script>
  import { onMount } from 'svelte';
  import { getChannels } from '../lib/api.js';
  import { channels } from '../lib/stores.js';

  let { currentTab = 'chat', switchTab, onLogout } = $props();

  let unreadCounts = $state({});

  const tabs = [
    { id: 'chat', icon: '💬', label: 'Чат' },
    { id: 'vpn', icon: '🌐', label: 'VPN' },
    { id: 'channels', icon: '📡', label: 'Каналы' },
    { id: 'identity', icon: '👤', label: 'Я' },
  ];

  async function loadChannels() {
    try {
      const data = await getChannels();
      const chList = data.channels || data || [];
      channels.set(chList);

      // Compute unread counts from API data
      const counts = {};
      for (const ch of chList) {
        if (ch.unread && ch.unread > 0) {
          counts[ch.id] = ch.unread;
        }
      }
      unreadCounts = counts;
    } catch (e) {
      // Fallback: keep existing store data
    }
  }

  let totalUnread = $derived(
    Object.values(unreadCounts).reduce((sum, n) => sum + n, 0)
  );

  onMount(() => {
    loadChannels();
    // Refresh channels periodically
    const interval = setInterval(loadChannels, 30000);
    return () => clearInterval(interval);
  });
</script>

<nav class="sidebar">
  <div class="logo" onclick={() => switchTab('identity')}>🔥</div>
  {#each tabs as tab}
    <div class="tab-wrap">
      <button
        class="tab"
        class:active={currentTab === tab.id}
        onclick={() => switchTab(tab.id)}
        title={tab.label}
      >
        <span class="icon">{tab.icon}</span>
        <span class="label">{tab.label}</span>
        {#if tab.id === 'channels' && totalUnread > 0}
          <span class="badge">{totalUnread > 99 ? '99+' : totalUnread}</span>
        {/if}
      </button>
    </div>
  {/each}

  {#if onLogout}
    <div class="tab-wrap logout-wrap">
      <button class="tab" onclick={onLogout} title="Выход">
        <span class="icon">🚪</span>
        <span class="label">Выход</span>
      </button>
    </div>
  {/if}
</nav>

<style>
  .sidebar {
    width: 72px;
    background: #111;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 16px 0;
    gap: 4px;
    border-right: 1px solid #222;
    flex-shrink: 0;
  }
  .logo {
    font-size: 28px;
    margin-bottom: 12px;
    cursor: pointer;
  }
  .tab-wrap {
    position: relative;
  }
  .tab {
    width: 56px;
    height: 56px;
    background: none;
    border: none;
    color: #666;
    border-radius: 12px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    transition: all 0.2s;
  }
  .tab:hover {
    background: #1a1a1a;
    color: #aaa;
  }
  .tab.active {
    background: #1e3a5f;
    color: #4fc3f7;
  }
  .icon {
    font-size: 20px;
  }
  .label {
    font-size: 9px;
  }
  .badge {
    position: absolute;
    top: 4px;
    right: 4px;
    background: #c62828;
    color: #fff;
    font-size: 9px;
    min-width: 16px;
    height: 16px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 4px;
    font-weight: 600;
  }
  .logout-wrap {
    margin-top: auto;
  }
  @media (max-width: 600px) {
    .sidebar {
      width: 56px;
      padding: 8px 0;
    }
    .tab {
      width: 44px;
      height: 44px;
    }
    .label {
      display: none;
    }
    .logo {
      font-size: 22px;
      margin-bottom: 8px;
    }
  }
</style>
