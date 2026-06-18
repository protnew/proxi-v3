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
  <div class="logo-wrap" onclick={() => switchTab('identity')}>
    <div class="logo">🔥</div>
    <div class="version">v1.2.0 (ef4855a)</div>
  </div>
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
    background: var(--bg-panel);
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 24px 0;
    gap: 12px;
    border-right: 1px solid var(--border-glass);
    flex-shrink: 0;
    box-shadow: 2px 0 15px var(--shadow-glass);
    z-index: 50;
  }
  .logo-wrap {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-bottom: 20px;
    cursor: pointer;
  }
  .logo {
    font-size: 32px;
    transition: transform 0.2s, filter 0.2s;
  }
  .logo-wrap:hover .logo {
    transform: scale(1.1);
    filter: drop-shadow(0 0 10px rgba(255, 100, 100, 0.6));
  }
  .version {
    font-size: 9px;
    color: var(--text-muted);
    margin-top: 4px;
    font-family: monospace;
    text-align: center;
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
    border: 1px solid transparent;
    color: var(--text-muted);
    border-radius: 16px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    transition: all 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .tab:hover {
    background: var(--bg-glass);
    border-color: var(--border-glass);
    color: var(--text-primary);
    transform: translateY(-2px);
  }
  .tab.active {
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: #ffffff; /* Active button text is always white regardless of theme */
    box-shadow: 0 4px 15px rgba(2, 136, 209, 0.4);
    border-color: transparent;
  }
  .icon {
    font-size: 22px;
  }
  .label {
    font-size: 11px;
    font-weight: 500;
  }
  .badge {
    position: absolute;
    top: -4px;
    right: 4px;
    background: var(--error);
    color: #ffffff;
    font-size: 10px;
    min-width: 20px;
    height: 20px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 4px;
    font-weight: 700;
    box-shadow: 0 2px 8px rgba(211, 47, 47, 0.5);
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
