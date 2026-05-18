<script>
  let { activeTab = $bindable('chat') } = $props();

  const tabs = [
    { id: 'chat', icon: '💬', label: 'Чат' },
    { id: 'vpn', icon: '🌐', label: 'VPN' },
    { id: 'channels', icon: '📡', label: 'Каналы' },
  ];
</script>

<nav class="sidebar">
  <div class="logo">🔥</div>
  {#each tabs as tab}
    <button
      class="tab {activeTab === tab.id ? 'active' : ''}"
      onclick={() => activeTab = tab.id}
      title={tab.label}
    >
      <span class="icon">{tab.icon}</span>
      <span class="label">{tab.label}</span>
    </button>
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
    gap: 8px;
    border-right: 1px solid #222;
  }
  .logo { font-size: 28px; margin-bottom: 24px; }
  .tab {
    width: 56px; height: 56px;
    background: none; border: none; color: #666;
    border-radius: 12px; cursor: pointer;
    display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    gap: 2px; transition: all 0.2s;
  }
  .tab:hover { background: #1a1a1a; color: #aaa; }
  .tab.active { background: #1e3a5f; color: #4fc3f7; }
  .icon { font-size: 20px; }
  .label { font-size: 9px; }
</style>
