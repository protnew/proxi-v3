<script>
  export let vpnStatus;
  export let toggleVpn;

  const peers = [
    // Mock data — will come from Netbird/Go backend
    // { name: 'Дима', ip: '10.0.0.2', status: 'online', isExitNode: true },
  ];
</script>

<div class="vpn-panel">
  <div class="header">
    <h1>🌐 Поделись интернет</h1>
    <div class="status" class:connected={vpnStatus === 'connected'} class:connecting={vpnStatus === 'connecting'}>
      {#if vpnStatus === 'connected'}🟢 Подключено{:else if vpnStatus === 'connecting'}🟡 Подключение...{:else}🔴 Отключено{/if}
    </div>
  </div>

  <div class="share-section">
    <button class="share-btn" class:active={vpnStatus === 'connected'} on:click={toggleVpn}>
      {#if vpnStatus === 'connected'}
        ⏹ Остановить точку
      {:else}
        🚀 Поделись интернетом
      {/if}
    </button>
    <p class="hint">Нажми — друзья смогут подключиться через тебя</p>
  </div>

  <div class="peers">
    <h3>Друзья онлайн</h3>
    {#if peers.length === 0}
      <div class="empty">Добавь друзей в чате — они появятся здесь</div>
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
  .share-section { text-align: center; margin: 40px 0; }
  .share-btn {
    font-size: 18px; padding: 16px 40px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: white; border: none; border-radius: 16px;
    cursor: pointer; transition: all 0.3s;
  }
  .share-btn:hover { transform: scale(1.05); }
  .share-btn.active { background: linear-gradient(135deg, #c62828, #b71c1c); }
  .hint { margin-top: 12px; color: #666; font-size: 13px; }
  .peers { margin-top: 32px; }
  .peers h3 { font-size: 14px; color: #888; margin-bottom: 16px; }
  .empty { color: #444; font-size: 13px; padding: 24px; text-align: center; }
</style>
