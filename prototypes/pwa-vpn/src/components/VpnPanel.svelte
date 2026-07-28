<script lang="ts">
  import {
    vpnStatus, vpnStats,
    connectRealVPN, connectLocalVPN, connectExitVPN, shareExitNode,
    disconnectVPN, refreshVPNStatus, checkEgressIP,
    formatBytes, formatUptime,
  } from '../lib/vpn'

  let expanded = $state(true)
  let mode = $state<'real' | 'exit' | 'share' | 'local'>('real')
  let exitPub = $state('')
  let exitEndpoint = $state('')
  let busy = $state(false)
  let checking = $state(false)

  let status = $state('disconnected')
  let stats = $state({
    bytesIn: 0, bytesOut: 0, uptime: 0, peers: 0,
    myIP: '', myPublicKey: '', transport: '', socksAddr: '', upstream: '',
    mode: '', realTraffic: false, lastError: '', egressIP: '',
  })

  vpnStatus.subscribe(v => { status = v })
  vpnStats.subscribe(v => { stats = v as typeof stats })

  const active = $derived(status === 'connected' || status === 'sharing')

  function statusLabel(s: string): string {
    switch (s) {
      case 'connected': return stats.realTraffic ? 'ON · SOCKS' : 'Подключён'
      case 'sharing': return 'Раздаю'
      case 'connecting': return 'Подключение…'
      case 'error': return 'Ошибка'
      default: return 'Отключён'
    }
  }

  async function primaryAction() {
    if (busy) return
    busy = true
    try {
      if (active) { await disconnectVPN(); return }
      if (mode === 'real') await connectRealVPN('')
      else if (mode === 'local') await connectLocalVPN()
      else if (mode === 'share') await shareExitNode()
      else await connectExitVPN(exitPub.trim(), exitEndpoint.trim())
    } finally {
      busy = false
      await refreshVPNStatus()
    }
  }

  async function onCheckIP() {
    checking = true
    try {
      await checkEgressIP()
    } catch (e) {
      stats = { ...stats, lastError: e instanceof Error ? e.message : String(e) }
    } finally {
      checking = false
      await refreshVPNStatus()
    }
  }

  function copySocks() {
    if (stats.socksAddr) navigator.clipboard.writeText(stats.socksAddr)
  }
</script>

<div class="vpn-panel" class:expanded class:on={active} class:real={stats.realTraffic}>
  <button type="button" class="vpn-bar" onclick={() => expanded = !expanded}>
    <span class="icon">🛡️</span>
    <span class="label">VPN</span>
    <span class="status" class:on={active} class:connecting={status === 'connecting'} class:err={status === 'error'}>
      {statusLabel(status)}
      {#if stats.transport}<span class="tr"> · {stats.transport}</span>{/if}
    </span>
    <span class="chevron">{expanded ? '▼' : '▲'}</span>
  </button>

  {#if expanded}
    <div class="vpn-details">
      <p class="desc">
        <strong>Настоящий VPN (Windows)</strong><br />
        Режим <em>SOCKS5</em> поднимает прокси <code>127.0.0.1:10808</code>. Браузер/приложение шлёт трафик туда — байты считаются, IP можно проверить кнопкой.<br />
        Системные маршруты Windows не переписываем (нужен admin + Wintun — отдельно).<br />
        <em>Exit</em>: укажи upstream SOCKS <code>host:port</code> (VPS/Tor) — цепочка через него.
      </p>

      {#if !active}
        <div class="modes">
          <label class:sel={mode === 'real'}><input type="radio" bind:group={mode} value="real" /> Настоящий SOCKS5</label>
          <label class:sel={mode === 'exit'}><input type="radio" bind:group={mode} value="exit" /> Через exit SOCKS</label>
          <label class:sel={mode === 'share'}><input type="radio" bind:group={mode} value="share" /> Раздать</label>
          <label class:sel={mode === 'local'}><input type="radio" bind:group={mode} value="local" /> Только статус (тест)</label>
        </div>
      {/if}

      {#if mode === 'exit' && !active}
        <div class="setup">
          <label for="exit-ep">Upstream SOCKS5 (host:port)</label>
          <input id="exit-ep" type="text" placeholder="vps.example.com:1080 или 127.0.0.1:9050 (Tor)" bind:value={exitEndpoint} />
          <label for="exit-pub">Peer pubkey (опц., для WG mesh)</label>
          <input id="exit-pub" type="text" placeholder="можно пусто — будет SOCKS upstream" bind:value={exitPub} />
        </div>
      {/if}

      {#if active}
        <div class="stats">
          <div class="stat"><span class="sl">Режим</span><span class="sv">{stats.mode || '—'} {stats.realTraffic ? '· REAL' : ''}</span></div>
          <div class="stat"><span class="sl">Транспорт</span><span class="sv">{stats.transport || '—'}</span></div>
          <div class="stat"><span class="sl">↓ / ↑</span><span class="sv">{formatBytes(stats.bytesIn)} / {formatBytes(stats.bytesOut)}</span></div>
          <div class="stat"><span class="sl">⏱</span><span class="sv">{formatUptime(stats.uptime)}</span></div>
        </div>

        {#if stats.socksAddr}
          <div class="socks-row">
            <span class="sl">SOCKS5</span>
            <code>{stats.socksAddr}</code>
            <button type="button" class="copy" onclick={copySocks}>📋</button>
          </div>
          <p class="hint ok">
            В Chrome: Settings → System → Open proxy settings → вручную SOCKS5 <code>{stats.socksAddr}</code><br />
            Или: <code>curl --socks5 {stats.socksAddr} https://api.ipify.org</code>
          </p>
        {/if}

        {#if stats.egressIP}
          <div class="ip-box">IP через туннель: <strong>{stats.egressIP}</strong></div>
        {/if}

        {#if stats.realTraffic}
          <button type="button" class="check-btn" disabled={checking} onclick={onCheckIP}>
            {checking ? '⏳ Проверяю IP…' : '🌐 Проверить IP через VPN'}
          </button>
        {/if}
      {/if}

      {#if stats.lastError}
        <div class="err-box">{stats.lastError}</div>
      {/if}

      <button type="button" class="toggle-btn" class:on={active} disabled={busy || status === 'connecting'} onclick={primaryAction}>
        {#if busy || status === 'connecting'}
          ⏳ Работаю…
        {:else if active}
          🛑 Отключить VPN
        {:else if mode === 'real'}
          🚀 Включить настоящий SOCKS5
        {:else if mode === 'exit'}
          🔗 Подключить через exit
        {:else if mode === 'share'}
          📡 Раздать + SOCKS
        {:else}
          🧪 Только статус (без трафика)
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .vpn-panel { background: #0e1621; border-top: 1px solid #1a2533; }
  .vpn-panel.on { border-top-color: #2a6a3a; }
  .vpn-panel.real.on { border-top-color: #3b82f6; }
  .vpn-bar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; font-size: 13px; width: 100%; background: none; border: none; color: inherit; font-family: inherit; text-align: left; }
  .vpn-bar:hover { background: #131d2a; }
  .icon { font-size: 16px; }
  .label { font-weight: 600; }
  .status { flex: 1; text-align: right; font-size: 12px; color: #8a9aaa; }
  .status.on { color: #4fae4e; }
  .status.connecting { color: #ffaa00; }
  .status.err { color: #ff6b6b; }
  .tr { opacity: 0.75; }
  .chevron { color: #555; font-size: 10px; }
  .vpn-details { padding: 0 12px 12px; }
  .desc { font-size: 11px; color: #9aabbb; line-height: 1.45; margin: 0 0 10px; }
  .desc code { font-size: 10px; color: #cde; }
  .modes { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
  .modes label { font-size: 11px; color: #c0c8d0; background: #17212b; padding: 6px 8px; border-radius: 8px; cursor: pointer; border: 1px solid transparent; }
  .modes label.sel { border-color: #3b82f6; color: #fff; }
  .modes input { margin-right: 4px; }
  .setup { margin-bottom: 10px; display: flex; flex-direction: column; gap: 4px; }
  .setup label { font-size: 11px; color: #7a8a9a; }
  .setup input { width: 100%; background: #242f3d; border: none; color: #e0e0e0; padding: 8px; border-radius: 6px; font-size: 12px; box-sizing: border-box; }
  .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 10px; }
  .stat { background: #17212b; padding: 8px; border-radius: 6px; display: flex; flex-direction: column; gap: 2px; }
  .sl { font-size: 11px; color: #7a8a9a; }
  .sv { font-size: 13px; font-weight: 600; color: #e0e0e0; word-break: break-all; }
  .socks-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; font-size: 12px; }
  .socks-row code { flex: 1; color: #7dd3fc; }
  .copy { background: none; border: 1px solid #333; border-radius: 6px; cursor: pointer; }
  .hint { font-size: 10px; color: #6a7a8a; margin: 0 0 10px; line-height: 1.4; }
  .hint.ok { color: #7aaf7a; }
  .hint code { font-size: 10px; color: #cde; }
  .ip-box { background: #0f2a1a; color: #6ee7b7; padding: 8px; border-radius: 6px; margin-bottom: 8px; font-size: 13px; }
  .err-box { background: #3a1a1a; color: #ff8a8a; font-size: 11px; padding: 8px; border-radius: 6px; margin-bottom: 8px; word-break: break-word; }
  .check-btn { width: 100%; margin-bottom: 8px; padding: 8px; border-radius: 8px; border: 1px solid #2a4a6a; background: #17212b; color: #7dd3fc; font-weight: 600; cursor: pointer; font-family: inherit; }
  .check-btn:disabled { opacity: 0.6; }
  .toggle-btn { width: 100%; padding: 10px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; background: #1e3a5f; color: #7dd3fc; font-family: inherit; }
  .toggle-btn.on { background: #3a1a1a; color: #ff6b6b; }
  .toggle-btn:disabled { opacity: 0.6; cursor: wait; }
</style>
