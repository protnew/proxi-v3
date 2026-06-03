<script>
  let { currentTab = 'chat', switchTab } = $props();

  const tabs = [
    { id: 'chat', icon: '💬', label: 'Чат' },
    { id: 'vpn', icon: '🌐', label: 'VPN' },
    { id: 'channels', icon: '📡', label: 'Каналы' },
    { id: 'identity', icon: '👤', label: 'Я' },
  ];
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
      </button>
    </div>
  {/each}
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
