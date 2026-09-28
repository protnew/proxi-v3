<script>
  import { onMount } from 'svelte';
  import { initApi, api } from '../lib/api.js';

  let vpnStatus = $state('disconnected');
  let myPublicKey = $state('');
  let loading = $state(false);

  onMount(async () => {
    initApi();
    try {
      const status = await api('vpn_status');
      if (status && status.state) vpnStatus = status.state;
      const key = await api('vpn_get_public_key');
      myPublicKey = key || '';
    } catch (e) {
      vpnStatus = 'unavailable';
    }
  });

  async function shareInternet() {
    loading = true;
    try {
      if (vpnStatus === 'sharing') {
        await api('vpn_stop_exit_node');
        vpnStatus = 'disconnected';
      } else {
        await api('vpn_start');
        await api('vpn_start_exit_node');
        vpnStatus = 'sharing';
      }
    } catch (e) {
      vpnStatus = 'error';
    }
    loading = false;
  }

  function copyKey() {
    if (myPublicKey) navigator.clipboard.writeText(myPublicKey);
  }
</script>

<div class="vpn-panel">
  <div class="header">
    <h1>🌐 Поделись интернет</h1>
    <div>
      {#if vpnStatus === 'sharing' || vpnStatus === 'connected'}
        <span class="status connected">🟢 {vpnStatus === 'sharing' ? 'Делю интернет' : 'Подключён'}</span>
      {:else if vpnStatus === 'unavailable'}
        <span class="status error">⚠️ Только в десктоп-приложении</span>
      {:else if vpnStatus === 'error'}
        <span class="status error">🔴 Ошибка</span>
      {:else}
        <span class="status">⚪ Отключено</span>
      {/if}
    </div>
  </div>

  <div class="share-section">
    <button class="share-btn {vpnStatus === 'sharing' ? 'active' : ''}" onclick={shareInternet} disabled={loading}>
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
        <button class="copy-btn" onclick={copyKey}>📋</button>
      </div>
      <p class="hint">Поделись с другом — он добавит тебя как пир</p>
    </div>
  {/if}

  <div class="peers">
    <h3>Друзья онлайн</h3>
    <div class="empty">Добавь друзей — их ключи появятся здесь</div>
  </div>
</div>

<style>
  .vpn-panel { padding: 32px; max-width: 500px; }
  .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 32px; }
  .header h1 { font-size: 22px; font-weight: 600; }
  .status { font-size: 14px; }
  .status.connected { color: #4caf50; }
  .status.error { color: #f44336; }
  .share-section { text-align: center; margin: 40px 0; }
  .share-btn {
    font-size: 18px; padding: 16px 40px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #fff; border: none; border-radius: 16px;
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
