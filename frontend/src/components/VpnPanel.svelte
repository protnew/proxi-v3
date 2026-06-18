<script>
  import { onMount, onDestroy } from 'svelte';
  import { showToast, contacts } from '../lib/stores.js';
  import { apiFetch } from '../lib/api.js';

  let vpnSharing = $state(false);
  let vpnSeconds = $state(0);
  let vpnTimer = null;
  let connectingTo = $state(null); // publicKey of node we are connecting to
  let isConnectedToNode = $state(false);

  function toggleVpn() {
    if (vpnSharing) {
      vpnSharing = false;
      clearInterval(vpnTimer);
      vpnSeconds = 0;
      showToast('Точка доступа остановлена');
      apiFetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'stop_exit_node', params: {} }),
      }).catch(() => {});
    } else {
      vpnSharing = true;
      apiFetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'start_exit_node', params: {} }),
      }).catch(() => {});

      setTimeout(() => {
        showToast('✅ Точка доступа запущена!');
        vpnTimer = setInterval(() => {
          vpnSeconds++;
        }, 1000);
      }, 1500);
    }
  }

  async function connectToNode(c) {
    if (isConnectedToNode && connectingTo === c.publicKey) {
      // Disconnect
      apiFetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'disconnect', params: {} }),
      }).catch(() => {});
      isConnectedToNode = false;
      connectingTo = null;
      showToast('Отключено от узла ' + c.name);
      return;
    }

    connectingTo = c.publicKey;
    showToast('Подключение к ' + c.name + '...');
    try {
      await apiFetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          method: 'connect_to_exit_node',
          params: { publicKey: c.publicKey, endpoint: c.endpoint }
        }),
      });
      showToast('✅ Подключено к ' + c.name);
      isConnectedToNode = true;
    } catch (e) {
      showToast('❌ ' + (e.message || 'Ошибка подключения'));
      connectingTo = null;
    }
  }

  // Derive separated lists
  let clients = $derived($contacts.filter(c => c.grantVpnAccess));
  let providers = $derived($contacts.filter(c => c.useAsVpnNode));

  onMount(() => {
    // Optionally fetch status from VPN RPC
  });
  
  onDestroy(() => {
    clearInterval(vpnTimer);
  });
</script>

<div class="vpn-panel">
  <div class="header-row">
    <h1>🌐 Управление интернетом</h1>
    <span class="status" class:off={!vpnSharing} class:ok={vpnSharing}>
      {#if vpnSharing}
        🟢 Делю интернет ({Math.floor(vpnSeconds / 60)}:{String(vpnSeconds % 60).padStart(2, '0')})
      {:else}
        ⚪ Отключено
      {/if}
    </span>
  </div>

  <div class="share-section">
    <button class="share-btn" class:active={vpnSharing} onclick={toggleVpn} disabled={false}>
      {vpnSharing ? '⏹ Остановить раздачу' : '🚀 Раздать интернет'}
    </button>
    <p class="hint">Ваши контакты с доступом "Дать интернет" смогут подключаться.</p>
  </div>

  <div class="section-card">
    <h3>📡 Кому я раздаю (Мои клиенты)</h3>
    {#if clients.length === 0}
      <div class="empty">У вас нет контактов с доступом к вашему VPN.</div>
    {:else}
      <div class="list">
        {#each clients as c}
          <div class="item">
            <div class="i-info">
              <span class="i-name">{c.name}</span>
              <span class="i-ep">{c.endpoint || 'Нет endpoint'}</span>
            </div>
            <div class="i-status ok">Доступ разрешён</div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <div class="section-card">
    <h3>🌍 Доступные VPN-узлы (Страны/Друзья)</h3>
    {#if providers.length === 0}
      <div class="empty">У вас нет контактов, чей интернет можно использовать.</div>
    {:else}
      <div class="list">
        {#each providers as c}
          <div class="item">
            <div class="i-info">
              <span class="i-name">{c.name}</span>
              <span class="i-ep">{c.endpoint || 'Нет endpoint'}</span>
            </div>
            <button class="action-btn" class:active={isConnectedToNode && connectingTo === c.publicKey} onclick={() => connectToNode(c)}>
              {#if isConnectedToNode && connectingTo === c.publicKey}
                Отключиться
              {:else}
                Подключиться
              {/if}
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .vpn-panel {
    padding: 24px;
    max-width: 600px;
    overflow-y: auto;
    height: 100%;
  }
  .header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
  }
  .header-row h1 {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .status {
    font-size: 13px;
    padding: 6px 12px;
    border-radius: 8px;
    background: var(--bg-glass);
    border: 1px solid var(--border-glass);
    color: var(--text-secondary);
  }
  .status.ok { color: var(--success); border-color: var(--success); background: rgba(129, 199, 132, 0.1); }
  .status.off { color: var(--text-muted); }
  
  .share-section {
    text-align: center;
    margin: 32px 0;
  }
  .share-btn {
    font-size: 16px;
    padding: 14px 36px;
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: #ffffff;
    border: none;
    border-radius: 14px;
    cursor: pointer;
    transition: all 0.3s;
    font-weight: 600;
  }
  .share-btn:hover {
    transform: scale(1.05);
    box-shadow: 0 4px 20px rgba(2, 136, 209, 0.4);
  }
  .share-btn.active {
    background: linear-gradient(135deg, var(--error), #c62828);
    box-shadow: 0 4px 20px rgba(229, 57, 53, 0.3);
  }
  .hint {
    margin-top: 10px;
    color: var(--text-muted);
    font-size: 12px;
  }

  .section-card {
    background: var(--bg-panel);
    border: 1px solid var(--border-glass);
    border-radius: 14px;
    padding: 20px;
    margin-bottom: 20px;
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
    box-shadow: var(--shadow-glass);
  }
  .section-card h3 {
    font-size: 14px;
    color: var(--text-secondary);
    margin-bottom: 16px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 600;
  }
  .empty {
    color: var(--text-muted);
    text-align: center;
    padding: 20px;
    font-size: 14px;
    background: var(--bg-glass);
    border-radius: 10px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-body);
    border: 1px solid var(--border-glass);
    border-radius: 10px;
    padding: 12px 16px;
    transition: all 0.2s;
  }
  .item:hover {
    border-color: var(--accent);
  }
  .i-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .i-name {
    font-weight: 600;
    font-size: 15px;
    color: var(--text-primary);
  }
  .i-ep {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
  }
  .i-status.ok {
    font-size: 12px;
    color: var(--success);
    background: rgba(129, 199, 132, 0.1);
    padding: 4px 8px;
    border-radius: 6px;
  }
  .action-btn {
    padding: 8px 16px;
    background: var(--bg-glass);
    color: var(--accent);
    border: 1px solid var(--border-glass);
    border-radius: 8px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
    transition: all 0.2s;
  }
  .action-btn:hover {
    background: var(--border-strong);
    color: var(--text-primary);
  }
  .action-btn.active {
    background: rgba(244, 67, 54, 0.1);
    color: var(--error);
    border-color: rgba(244, 67, 54, 0.3);
  }
  .action-btn.active:hover {
    background: rgba(244, 67, 54, 0.2);
    color: #ffffff;
  }
</style>
