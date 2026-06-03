<script>
  import { onMount } from 'svelte';
  import { peers, showToast, escHtml } from '../lib/stores.js';

  let vpnSharing = $state(false);
  let vpnSeconds = $state(0);
  let vpnTimer = null;
  let peerName = $state('');
  let peerPubKey = $state('');
  let peerEndpoint = $state('');

  function toggleVpn() {
    if (vpnSharing) {
      vpnSharing = false;
      clearInterval(vpnTimer);
      vpnSeconds = 0;
      showToast('Точка доступа остановлена');
      fetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'stop_exit_node' }),
      }).catch(() => {});
    } else {
      vpnSharing = true;
      fetch('/api/vpn/rpc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: 'start_exit_node' }),
      }).catch(() => {});

      setTimeout(() => {
        showToast('✅ Точка доступа запущена!');
        vpnTimer = setInterval(() => {
          vpnSeconds++;
          if (vpnSeconds % 10 === 0) loadPeers();
        }, 1000);
      }, 1500);
    }
  }

  async function loadPeers() {
    try {
      const r = await fetch('/api/peers');
      const d = await r.json();
      peers.set(d.peers || []);
    } catch (e) {}
  }

  async function addPeerFromForm() {
    if (!peerPubKey.trim()) {
      showToast('❌ Публичный ключ обязателен');
      return;
    }
    try {
      const r = await fetch('/api/peers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: peerName.trim(),
          publicKey: peerPubKey.trim(),
          endpoint: peerEndpoint.trim(),
        }),
      });
      const d = await r.json();
      if (!r.ok) {
        showToast('❌ ' + (d.error?.message || 'Ошибка'));
        return;
      }
      peers.set(d.peers || []);
      peerName = '';
      peerPubKey = '';
      peerEndpoint = '';
      showToast('✅ ' + (peerName || 'Пир') + ' добавлен!');
    } catch (e) {
      showToast('❌ Ошибка добавления пира');
    }
  }

  async function removePeer(peerId) {
    if (!confirm('Удалить пира ' + peerId + '?')) return;
    try {
      const r = await fetch('/api/peers', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ peerId }),
      });
      const d = await r.json();
      if (!r.ok) {
        showToast('❌ ' + (d.error?.message || 'Ошибка'));
        return;
      }
      peers.set(d.peers || []);
      showToast('🗑 Пир удалён');
    } catch (e) {
      showToast('❌ Ошибка удаления пира');
    }
  }

  onMount(() => {
    loadPeers();
  });
</script>

<div class="vpn-panel">
  <div class="header-row">
    <h1>🌐 Поделись интернет</h1>
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
      {vpnSharing ? '⏹ Остановить точку' : '🚀 Поделись интернетом'}
    </button>
    <p class="hint">Нажми — друзья смогут подключиться через тебя</p>
  </div>

  <div class="id-card" style="margin-top:8px">
    <h3>➕ Добавить друга</h3>
    <div style="margin-bottom:10px">
      <input bind:value={peerName} placeholder="Имя друга" class="form-input" />
    </div>
    <div style="margin-bottom:10px">
      <input bind:value={peerPubKey} placeholder="Публичный ключ" class="form-input mono" />
    </div>
    <div style="margin-bottom:12px">
      <input bind:value={peerEndpoint} placeholder="Endpoint (ip:port)" class="form-input" />
    </div>
    <button onclick={addPeerFromForm} class="submit-btn">Добавить</button>
  </div>

  <div class="peers" style="margin-top:16px">
    <h3>Подключённые пиры</h3>
    {#if $peers.length === 0}
      <div class="empty-peers">Пока никто не подключён</div>
    {:else}
      {#each $peers as p}
        <div class="peer-item">
          <div class="peer-dot" class:offline={!p.online}></div>
          <div class="peer-name">{p.name || p.id}</div>
          <div class="peer-ip">{p.allowedIPs || p.endpoint || ''}</div>
          <button class="peer-remove" onclick={() => removePeer(p.id)}>✕</button>
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .vpn-panel {
    padding: 24px;
    max-width: 500px;
    overflow-y: auto;
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
  }
  .status {
    font-size: 13px;
    padding: 4px 10px;
    border-radius: 8px;
    background: #111;
  }
  .status.ok { color: #4caf50; }
  .status.off { color: #666; }
  .share-section {
    text-align: center;
    margin: 32px 0;
  }
  .share-btn {
    font-size: 16px;
    padding: 14px 36px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #fff;
    border: none;
    border-radius: 14px;
    cursor: pointer;
    transition: all 0.3s;
    font-weight: 500;
  }
  .share-btn:hover {
    transform: scale(1.05);
    box-shadow: 0 4px 20px rgba(30, 136, 229, 0.3);
  }
  .share-btn.active {
    background: linear-gradient(135deg, #c62828, #b71c1c);
  }
  .hint {
    margin-top: 10px;
    color: #555;
    font-size: 12px;
  }
  .id-card {
    background: #111;
    border-radius: 14px;
    padding: 20px;
    border: 1px solid #222;
    margin-bottom: 16px;
  }
  .id-card h3 {
    font-size: 13px;
    color: #888;
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .form-input {
    width: 100%;
    padding: 10px 14px;
    background: #0a0a0a;
    border: 1px solid #333;
    border-radius: 8px;
    color: #e0e0e0;
    font-size: 13px;
    outline: none;
  }
  .form-input.mono {
    font-family: monospace;
  }
  .submit-btn {
    width: 100%;
    padding: 12px;
    background: #1e88e5;
    color: #fff;
    border: none;
    border-radius: 10px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s;
  }
  .submit-btn:hover {
    background: #1565c0;
  }
  .peers {
    margin-top: 24px;
  }
  .peers h3 {
    font-size: 13px;
    color: #888;
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .empty-peers {
    color: #333;
    font-size: 12px;
    padding: 20px;
    text-align: center;
  }
  .peer-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px;
    background: #111;
    border-radius: 10px;
    margin-bottom: 8px;
    border: 1px solid #1a1a1a;
  }
  .peer-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #4caf50;
  }
  .peer-dot.offline {
    background: #555;
  }
  .peer-name {
    font-size: 13px;
    flex: 1;
  }
  .peer-ip {
    font-size: 11px;
    color: #555;
  }
  .peer-remove {
    padding: 4px 10px;
    background: #1a1a1a;
    border: 1px solid #333;
    color: #f44336;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    transition: all 0.2s;
  }
  .peer-remove:hover {
    background: #f44336;
    color: #fff;
    border-color: #f44336;
  }
</style>
