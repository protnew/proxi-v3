<script>
  import { onMount } from 'svelte';
  import { getChannels } from '../lib/api.js';
  import { channels } from '../lib/stores.js';
  import { icons } from '../lib/icons.js';

  let { currentTab = 'chat', switchTab, onLogout } = $props();

  let unreadCounts = $state({});

  const tabs = [
    { id: 'chat', icon: icons.chat, label: 'Чат', color: 'var(--accent-color)' },
    { id: 'contacts', icon: icons.contacts, label: 'Контакты', color: '#10b981' },
    { id: 'vpn', icon: icons.vpn, label: 'VPN', color: '#8b5cf6' },
    { id: 'channels', icon: icons.channels, label: 'Каналы', color: '#f59e0b' },
    { id: 'switch', icon: '💀', label: 'DMS', color: '#ef4444' },
    { id: 'identity', icon: icons.identity, label: 'Я', color: '#ec4899' },
    { id: 'mvp', icon: icons.mvp, label: 'MVP', color: '#06b6d4' },
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
    <div class="logo">{@html icons.logo}</div>
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
        <span class="icon" style="color: {tab.color}">{@html tab.icon}</span>
        <span class="label">{tab.label}</span>
        {#if tab.id === 'channels' && totalUnread > 0}
          <span class="badge">{totalUnread > 99 ? '99+' : totalUnread}</span>
        {/if}
      </button>
    </div>
  {/each}

  <div class="spacer" style="flex: 1;"></div>

  {#if onLogout}
    <div class="tab-wrap logout-wrap">
      <button class="tab" onclick={onLogout} title="Выход">
        <span class="icon">{@html icons.logout}</span>
        <span class="label">Выход</span>
      </button>
    </div>
  {/if}
</nav>

<style>
    .sidebar {
    width: 80px;
    background: var(--bg-panel);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 24px 0;
    gap: 12px;
    border-right: 1px solid var(--border-color);
    flex-shrink: 0;
    box-shadow: var(--shadow-md);
    z-index: 50;
    transition: var(--transition-normal);
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
  }
  .version {
    font-size: 9px;
    color: var(--text-secondary);
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
    border: none;
    color: var(--text-secondary);
    border-radius: var(--border-radius-md);
    cursor: pointer;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    transition: var(--transition-normal);
  }
  .tab:hover {
    background: var(--bg-panel-hover);
    color: var(--text-primary);
    transform: translateY(-2px);
  }
  .tab.active {
    background: var(--accent-color);
    color: #ffffff !important;
    box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
  }
  .tab.active .icon {
    color: #ffffff !important;
  }
  .icon {
    font-size: 22px;
    transition: var(--transition-normal);
  }
  .label {
    font-size: 11px;
    font-weight: 500;
  }
  .badge {
    position: absolute;
    top: -4px;
    right: 4px;
    background: #ef4444;
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
    box-shadow: var(--shadow-sm);
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
