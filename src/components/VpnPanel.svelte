<script>
  import { onMount } from 'svelte';
  import { invoke } from '@tauri-apps/api/core';

  export let vpnStatus;
  export let toggleVpn;

  let peers = [];
  let myPublicKey = '';
  let loading = false;

  onMount(async () => {
    try {
      const status = await invoke('vpn_status');
      if (status.state) {
        $state = status.state;
      }
      const key = await invoke('vpn_get_public_key');
      myPublicKey = key || '';
    } catch (e) {
      console.log('VPN not started yet:', e);
    }
  });

  async function shareInternet() {
    loading = true;
    try {
      if (vpnStatus === 'sharing') {
        await invoke('vpn_stop_exit_node');
        vpnStatus = 'disconnected';
      } else {
        // Start VPN process first if needed
        await invoke('vpn_start');
        await invoke('vpn_start_exit_node');
        vpnStatus = 'sharing';
      }
    } catch (e) {
      console.error('VPN error:', e);
      vpnStatus = 'error';
    }
    loading = false;
  }

  function copyKey() {
    navigator.clipboard.writeText(myPublicKey);
  }
</script>

<div class="vpn-panel">
  <div class="header">
    <h1>🌐 Поделись интернет</h1>
    <div class="status" class:connected={vpnStatus === 'sharing' || vpnStatus === 'connected'} class:connecting={vpnStatus === 'connecting'} class:error={vpnStatus === 'error'}>
      {#if vpnStatus === 'sharing'}🟢 Делю интернет{:else if vpnStatus === 'connected'}🟢 Подключён{:else if vpnStatus === 'connecting'}🟡 Подключение...{:else if vpnStatus === 'error'}🔴 Ошибка{:else}⚪ Отключено{/if}
    </div>
  </div>

  <div class="share-section">
    <button class="share-btn" class:active={vpnStatus === 'sharing'} on:click={shareInternet} disabled={loading}>
      {#if loading}
        ⏳ ...
      {:else if vpnStatus === 'sharing'}
        ⏹ Остановить точку
      {:else}
        🚀 Поделись интернетом
      {/if}
    </button>
    <p class="hint">Нажми — друзья смогут подключиться через тебя</p>
  </div>

  {#if myPublicKey}
    <div class="key-section">
      <h3>Твой публичный ключ</h3>
      <div class="key-row">
        <code class="key">{myPublicKey.substring(0, 20)}...{myPublicKey.substring(myPublicKey.length - 8)}</code>
        <button class="copy-btn" on:click={copyKey}>📋</button>
      </div>
      <p class="hint">Поделись с другом — он добавит тебя как пир</p>
    </div>
  {/if}

  <div class="peers">
    <h3>Друзья онлайн</h3>
    {#if peers.length === 0}
      <div class="empty">Добавь друзей — их публичные ключи появятся здесь</div>
    {/if}
  </div>
</div>

<style>
  .vpn-panel { padding: 32px; max-width: 500px; }
  .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 32px; }
  .header h1 { font-size: 22px; font-weight: 600; }
  .status { font-size: 14px; }
  .status.connected { color: #4caf50; }
  .status.connecting { color: #ff9800; }
  .status.error { color: #f44336; }
  .share-section { text-align: center; margin: 40px 0; }
  .share-btn {
    font-size: 18px; padding: 16px 40px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: white; border: none; border-radius: 16px;
    cursor: pointer; transition: all 0.3s;
  }
  .share-btn:hover { transform: scale(1.05); }
  .share-btn:disabled { opacity: 0.5; cursor: wait; }
  .share-btn.active { background: linear-gradient(135deg, #c62828, #b71c1c); }
  .hint { margin-top: 12px; color: #666; font-size: 13px; }
  .key-section { margin: 24px 0; padding: 16px; background: #111; border-radius: 12px; border: 1px solid #222; }
  .key-section h3 { font-size: 12px; color: #888; margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.5px; }
  .key-row { display: flex; align-items: center; gap: 8px; }
  .key { flex: 1; font-size: 11px; color: #4fc3f7; word-break: break-all; }
  .copy-btn { background: #222; border: 1px solid #333; color: #888; border-radius: 6px; padding: 4px 8px; cursor: pointer; }
  .copy-btn:hover { background: #333; }
  .peers { margin-top: 32px; }
  .peers h3 { font-size: 14px; color: #888; margin-bottom: 16px; }
  .empty { color: #444; font-size: 13px; padding: 24px; text-align: center; }
</style>
