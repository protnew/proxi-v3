<script>
  import { onMount } from 'svelte';
  import { getChannels } from '../lib/api.js';
  import { channels } from '../lib/stores.js';

  let { currentTab = 'chat', switchTab, onLogout } = $props();

  let unreadCounts = $state({});

  const tabs = [
    { id: 'chat', icon: '💬', label: 'Чат' },
    { id: 'contacts', icon: '👥', label: 'Контакты' },
    { id: 'vpn', icon: '🌐', label: 'VPN' },
    { id: 'channels', icon: '📡', label: 'Каналы' },
    { id: 'identity', icon: '👤', label: 'Я' },
    { id: 'mvp', icon: '🧪', label: 'MVP' },
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
    width: 80px;
    background: rgba(10, 10, 10, 0.9);
    backdrop-filter: blur(20px);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 20px 0;
    gap: 12px;
    border-right: 1px solid rgba(255, 255, 255, 0.1);
    flex-shrink: 0;
    box-shadow: 2px 0 10px rgba(0, 0, 0, 0.5);
    z-index: 50;
  }
  .logo {
    font-size: 32px;
    margin-bottom: 20px;
    cursor: pointer;
    transition: transform 0.2s;
  }
  .logo:hover {
    transform: scale(1.1);
  }
  .tab-wrap {
    position: relative;
    width: 100%;
    display: flex;
    justify-content: center;
  }
  .tab {
    width: 60px;
    height: 60px;
    background: transparent;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    border-radius: 16px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    transition: all 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .tab:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #ffffff;
    transform: translateY(-2px);
  }
  .tab.active {
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #ffffff;
    box-shadow: 0 4px 15px rgba(30, 136, 229, 0.4);
  }
  .icon {
    font-size: 22px;
  }
  .label {
    font-size: 10px;
    font-weight: 500;
  }
  .badge {
    position: absolute;
    top: -2px;
    right: 6px;
    background: #ff5252;
    color: #ffffff;
    font-size: 10px;
    min-width: 18px;
    height: 18px;
    border-radius: 9px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 4px;
    font-weight: 700;
    box-shadow: 0 2px 5px rgba(255, 82, 82, 0.5);
  }
  .logout-wrap {
    margin-top: auto;
  }
  @media (max-width: 600px) {
    .sidebar {
      width: 64px;
      padding: 12px 0;
    }
    .tab {
      width: 50px;
      height: 50px;
    }
    .label {
      display: none;
    }
    .logo {
      font-size: 24px;
      margin-bottom: 12px;
    }
  }
</style>
