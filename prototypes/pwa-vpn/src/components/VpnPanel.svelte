<script lang="ts">
  import { vpnStatus, vpnStats, connectVPN, disconnectVPN, formatBytes, formatUptime } from '../lib/vpn'

  let expanded = $state(false)
  let exitUrl = $state('')

  // Subscribe to stores
  let status = $state('disconnected')
  let stats = $state({ bytesIn: 0, bytesOut: 0, uptime: 0, peers: 0 })

  vpnStatus.subscribe(v => { status = v })
  vpnStats.subscribe(v => { stats = v })

  function toggle() {
    if (status === 'connected') {
      disconnectVPN()
    } else {
      connectVPN(exitUrl || undefined)
    }
  }
</script>

<div class="vpn-panel" class:expanded>
  <button class="vpn-bar" onclick={() => expanded = !expanded}>
    <span class="icon">🛡️</span>
    <span class="label">VPN</span>
    <span class="status" class:on={status === 'connected'} class:connecting={status === 'connecting'}>
      {#if status === 'connected'}Подключён{:else if status === 'connecting'}Подключение...{:else if status === 'error'}Ошибка{:else}Отключён{/if}
    </span>
    <span class="chevron">{expanded ? '▼' : '▲'}</span>
  </button>

  {#if expanded}
    <div class="vpn-details">
      {#if status === 'connected'}
        <div class="stats">
          <div class="stat"><span class="sl">↓ Входящий</span><span class="sv">{formatBytes(stats.bytesIn)}</span></div>
          <div class="stat"><span class="sl">↑ Исходящий</span><span class="sv">{formatBytes(stats.bytesOut)}</span></div>
          <div class="stat"><span class="sl">⏱ Время</span><span class="sv">{formatUptime(stats.uptime)}</span></div>
          <div class="stat"><span class="sl">👥 Пиры</span><span class="sv">{stats.peers}</span></div>
        </div>
      {:else}
        <div class="setup">
          <label for="exit-url">Exit Node URL (опционально)</label>
          <input id="exit-url" type="text" placeholder="wss://your-exit-node.com" bind:value={exitUrl} />
          <p class="hint">Без exit node используется прямое P2P соединение</p>
        </div>
      {/if}

      <button class="toggle-btn" class:on={status === 'connected'} onclick={toggle}>
        {#if status === 'connected'}
          🛑 Отключить VPN
        {:else if status === 'connecting'}
          ⏳ Подключение...
        {:else}
          🚀 Подключить VPN
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .vpn-panel { background: #0e1621; border-top: 1px solid #1a2533; }
  .vpn-bar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; font-size: 13px; width: 100%; background: none; border: none; color: inherit; font-family: inherit; text-align: left; }
  .vpn-bar:hover { background: #131d2a; }
  .icon { font-size: 16px; }
  .label { font-weight: 600; }
  .status { flex: 1; text-align: right; font-size: 12px; }
  .status.on { color: #4fae4e; }
  .status.connecting { color: #ffaa00; }
  .chevron { color: #555; font-size: 10px; }
  .vpn-details { padding: 0 12px 12px; }
  .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; }
  .stat { background: #17212b; padding: 8px; border-radius: 6px; display: flex; flex-direction: column; gap: 2px; }
  .sl { font-size: 11px; color: #7a8a9a; }
  .sv { font-size: 14px; font-weight: 600; color: #e0e0e0; }
  .setup { margin-bottom: 12px; }
  .setup label { display: block; font-size: 11px; color: #7a8a9a; margin-bottom: 4px; }
  .setup input { width: 100%; background: #242f3d; border: none; color: #e0e0e0; padding: 8px; border-radius: 6px; font-size: 12px; box-sizing: border-box; }
  .hint { font-size: 10px; color: #555; margin-top: 4px; }
  .toggle-btn { width: 100%; padding: 10px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; background: #2a4a3a; color: #4fae4e; font-family: inherit; }
  .toggle-btn.on { background: #3a1a1a; color: #ff6b6b; }
</style>
