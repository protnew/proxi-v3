<script lang="ts">
  import {
    vpnStatus, vpnStats,
    connectRealVPN, connectLocalVPN, connectExitVPN, shareExitNode,
    disconnectVPN, refreshVPNStatus, checkEgressIP, unlockVPN,
    formatBytes, formatUptime,
  } from '../lib/vpn'
  import { ERROR_LABELS, type VpnErrorCode } from '../lib/vpn-wire'

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
    lastErrorCode: '', phase: '', attempt: 0, max: 0,
  })

  function errLabel(code: string): string {
    const e = ERROR_LABELS[code as VpnErrorCode]
    return e ? e.ru : ''
  }

  vpnStatus.subscribe(v => { status = v })
  vpnStats.subscribe(v => { stats = v as typeof stats })

  const active = $derived(status === 'connected' || status === 'sharing')

  function statusLabel(s: string): string {
    switch (s) {
      case 'connected': return stats.realTraffic ? 'ON · SOCKS' : 'Подключён'
      case 'sharing': return 'Раздаю'
      case 'connecting': return 'Подключение…'
      case 'reconnecting': return stats.max ? `Повтор ${stats.attempt}/${stats.max}…` : 'Повтор…'
      case 'locked': return 'Заблокирован'
      case 'core_down': return 'Ядро недоступно'
      case 'helper_missing': return 'Нет службы'
      case 'disconnecting': return 'Отключение…'
      case 'error': return 'Ошибка'
      case 'off': return 'Отключён'
      default: return 'Ошибка'
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

  // Generate share link for friend: socks://host:port
  let shareUrl = $state('')
  $effect(() => {
    if (stats.socksAddr) {
      const host = window.location.hostname || '127.0.0.1'
      const port = stats.socksAddr.split(':').pop() || '10808'
      shareUrl = `socks5://${host}:${port}`
    }
  })

  function copyShareLink() {
    if (shareUrl) navigator.clipboard.writeText(shareUrl)
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

          {#if active && stats.socksAddr}
            <div class="share-box">
              <div class="share-title">📡 Поделиться VPN с другом</div>
              <div class="share-link-row">
                <input type="text" readonly value={shareUrl} class="share-input" />
                <button type="button" class="copy share-copy" onclick={copyShareLink}>📋</button>
              </div>
              <p class="hint">
                Отправь эту ссылку другу. Он откроет её, нажмёт «Подключиться» — и его трафик пойдёт через твой SOCKS5.
              </p>
            </div>
          {/if}

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

      {#if status === 'locked'}
        <div class="err-box">
          ⚠️ Клиент упал — туннель удерживается (locked).
          <button type="button" class="unlock-btn" onclick={unlockVPN}>Снять удержание туннеля</button>
        </div>
      {/if}

      {#if stats.lastError}
        <div class="err-box">
          {#if stats.lastErrorCode}<strong>{errLabel(stats.lastErrorCode)}</strong> · {/if}{stats.lastError}
        </div>
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
  .vpn-panel { background: var(--bg); border-top: 1px solid var(--border); }
  .vpn-panel.on { border-top-color: var(--success); }
  .vpn-panel.real.on { border-top-color: var(--accent); }
  .vpn-bar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; font-size: 13px; width: 100%; background: none; border: none; color: inherit; font-family: inherit; text-align: left; }
  .vpn-bar:hover { background: var(--bg-hover); }
  .icon { font-size: 16px; }
  .label { font-weight: 600; }
  .status { flex: 1; text-align: right; font-size: 12px; color: var(--text-muted); }
  .status.on { color: var(--success); }
  .status.connecting { color: var(--warn); }
  .status.err { color: var(--danger); }
  .tr { opacity: 0.75; }
  .chevron { color: var(--text-muted); font-size: 12px; }
  .vpn-details { padding: 0 12px 12px; }
  .desc { font-size: 12px; color: var(--text-muted); line-height: 1.45; margin: 0 0 10px; }
  .desc code { font-size: 12px; color: var(--text); }
  .modes { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
  .modes label { font-size: 12px; color: var(--text); background: var(--bg-secondary); padding: 6px 8px; border-radius: 8px; cursor: pointer; border: 1px solid transparent; }
  .modes label.sel { border-color: var(--accent); color: var(--text-on-accent); }
  .modes input { margin-right: 4px; }
  .setup { margin-bottom: 10px; display: flex; flex-direction: column; gap: 4px; }
  .setup label { font-size: 12px; color: var(--text-muted); }
  .setup input { width: 100%; background: var(--bg-tertiary); border: none; color: var(--text); padding: 8px; border-radius: 6px; font-size: 12px; box-sizing: border-box; }
  .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 10px; }
  .stat { background: var(--bg-secondary); padding: 8px; border-radius: 6px; display: flex; flex-direction: column; gap: 2px; }
  .sl { font-size: 12px; color: var(--text-muted); }
  .sv { font-size: 13px; font-weight: 600; color: var(--text); word-break: break-all; }
  .socks-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; font-size: 12px; }
  .socks-row code { flex: 1; color: var(--accent-light); }
  .copy { background: none; border: 1px solid var(--bg-tertiary); border-radius: 6px; cursor: pointer; }
  .hint { font-size: 12px; color: var(--text-muted); margin: 0 0 10px; line-height: 1.4; }
  .hint.ok { color: var(--success); }
  .hint code { font-size: 12px; color: var(--text); }
  .ip-box { background: color-mix(in srgb, var(--success) 12%, var(--bg)); color: var(--success); padding: 8px; border-radius: 6px; margin-bottom: 8px; font-size: 13px; }
  .err-box { background: color-mix(in srgb, var(--danger) 22%, var(--bg)); color: var(--danger); font-size: 12px; padding: 8px; border-radius: 6px; margin-bottom: 8px; word-break: break-word; }
  .unlock-btn { display: block; margin-top: 6px; padding: 6px 10px; border: 1px solid var(--danger); border-radius: 6px; background: none; color: var(--danger); cursor: pointer; font-family: inherit; font-size: 12px; }
  .check-btn { width: 100%; margin-bottom: 8px; padding: 8px; border-radius: 8px; border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--bg)); background: var(--bg-secondary); color: var(--accent-light); font-weight: 600; cursor: pointer; font-family: inherit; }
  .check-btn:disabled { opacity: 0.6; }
  .toggle-btn { width: 100%; padding: 10px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; background: var(--bg-secondary); color: var(--accent-light); font-family: inherit; }
  .toggle-btn.on { background: color-mix(in srgb, var(--danger) 22%, var(--bg)); color: var(--danger); }
  .toggle-btn:disabled { opacity: 0.6; cursor: wait; }
  .share-box { background: color-mix(in srgb, var(--success) 12%, var(--bg)); border: 1px solid var(--border); border-radius: 8px; padding: 10px; margin-bottom: 8px; }
  .share-title { font-size: 12px; font-weight: 600; color: var(--success); margin-bottom: 6px; }
  .share-link-row { display: flex; gap: 8px; }
  .share-input { flex: 1; background: var(--bg-secondary); border: 1px solid var(--border); color: var(--success); padding: 6px 8px; border-radius: 6px; font-size: 12px; font-family: monospace; }
  .share-copy { flex: none; }
</style>
