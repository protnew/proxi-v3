<script lang="ts">
  import {
    vpnStatus, vpnStats,
    connectLocalVPN, connectExitVPN, shareExitNode, disconnectVPN, refreshVPNStatus,
    formatBytes, formatUptime,
  } from '../lib/vpn'

  let expanded = $state(true)
  let mode = $state<'local' | 'exit' | 'share'>('local')
  let exitPub = $state('')
  let exitEndpoint = $state('')
  let busy = $state(false)

  let status = $state('disconnected')
  let stats = $state({
    bytesIn: 0, bytesOut: 0, uptime: 0, peers: 0,
    myIP: '', myPublicKey: '', transport: '', lastError: '', mode: '' as string,
  })

  vpnStatus.subscribe(v => { status = v })
  vpnStats.subscribe(v => { stats = v as typeof stats })

  const active = $derived(status === 'connected' || status === 'sharing')

  function statusLabel(s: string): string {
    switch (s) {
      case 'connected': return 'Подключён'
      case 'sharing': return 'Раздаю (exit)'
      case 'connecting': return 'Подключение…'
      case 'error': return 'Ошибка'
      default: return 'Отключён'
    }
  }

  async function primaryAction() {
    if (busy) return
    busy = true
    try {
      if (active) {
        await disconnectVPN()
        return
      }
      if (mode === 'local') {
        await connectLocalVPN()
      } else if (mode === 'share') {
        await shareExitNode()
      } else {
        await connectExitVPN(exitPub.trim(), exitEndpoint.trim())
      }
    } finally {
      busy = false
      await refreshVPNStatus()
    }
  }

  function copyPub() {
    if (!stats.myPublicKey) return
    navigator.clipboard.writeText(stats.myPublicKey)
  }
</script>

<div class="vpn-panel" class:expanded class:on={active}>
  <button type="button" class="vpn-bar" onclick={() => expanded = !expanded}>
    <span class="icon">🛡️</span>
    <span class="label">VPN</span>
    <span class="status" class:on={active} class:connecting={status === 'connecting'} class:err={status === 'error'}>
      {statusLabel(status)}
      {#if stats.transport}
        <span class="tr">· {stats.transport}</span>
      {/if}
    </span>
    <span class="chevron">{expanded ? '▼' : '▲'}</span>
  </button>

  {#if expanded}
    <div class="vpn-details">
      <p class="desc">
        <strong>Как это работает</strong><br />
        1) <em>Локальный туннель</em> — тестовый режим на этом ПК (userspace/stub). Не меняет маршруты Windows, можно жать сразу.<br />
        2) <em>К exit-node</em> — нужен pubkey (≥16) и endpoint <code>host:port</code> друга/сервера.<br />
        3) <em>Раздать интернет</em> — стать exit-node для других (на Windows без WireGuard = userspace/stub).
      </p>

      {#if !active}
        <div class="modes">
          <label class:sel={mode === 'local'}><input type="radio" bind:group={mode} value="local" /> Локальный тест</label>
          <label class:sel={mode === 'exit'}><input type="radio" bind:group={mode} value="exit" /> К exit-node</label>
          <label class:sel={mode === 'share'}><input type="radio" bind:group={mode} value="share" /> Раздать</label>
        </div>
      {/if}

      {#if mode === 'exit' && !active}
        <div class="setup">
          <label for="exit-pub">Public key exit-node</label>
          <input id="exit-pub" type="text" placeholder="base64/hex ключ (≥16 символов)" bind:value={exitPub} />
          <label for="exit-ep">Endpoint</label>
          <input id="exit-ep" type="text" placeholder="example.com:51820" bind:value={exitEndpoint} />
        </div>
      {/if}

      {#if active}
        <div class="stats">
          <div class="stat"><span class="sl">IP</span><span class="sv">{stats.myIP || '—'}</span></div>
          <div class="stat"><span class="sl">Транспорт</span><span class="sv">{stats.transport || '—'}</span></div>
          <div class="stat"><span class="sl">↓ / ↑</span><span class="sv">{formatBytes(stats.bytesIn)} / {formatBytes(stats.bytesOut)}</span></div>
          <div class="stat"><span class="sl">⏱ / пиры</span><span class="sv">{formatUptime(stats.uptime)} · {stats.peers}</span></div>
        </div>
        {#if stats.myPublicKey}
          <div class="pubkey-row">
            <span class="sl">Мой VPN key</span>
            <code>{stats.myPublicKey.slice(0, 18)}…</code>
            <button type="button" class="copy" onclick={copyPub}>📋</button>
          </div>
        {/if}
        <p class="hint ok">
          {#if stats.transport === 'stub'}
            Stub-режим: UI и API статуса работают, системный трафик Windows не перехватывается (нет wg/TUN).
          {:else if stats.transport === 'userspace'}
            Userspace UDP-туннель поднят. Полный system-wide VPN — только с peer/exit и ОС-маршрутами.
          {:else}
            Туннель активен ({stats.transport || 'unknown'}).
          {/if}
        </p>
      {:else}
        <p class="hint">
          Messenger (чаты) работает отдельно от VPN. VPN-кнопка управляет бэкендом <code>/api/vpn/rpc</code>.
        </p>
      {/if}

      {#if stats.lastError}
        <div class="err-box">{stats.lastError}</div>
      {/if}

      <button type="button" class="toggle-btn" class:on={active} disabled={busy || status === 'connecting'} onclick={primaryAction}>
        {#if busy || status === 'connecting'}
          ⏳ Работаю…
        {:else if active}
          🛑 Отключить VPN
        {:else if mode === 'local'}
          🚀 Включить локальный туннель
        {:else if mode === 'share'}
          📡 Начать раздачу (exit)
        {:else}
          🔗 Подключить к exit-node
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .vpn-panel { background: #0e1621; border-top: 1px solid #1a2533; }
  .vpn-panel.on { border-top-color: #2a6a3a; }
  .vpn-bar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; font-size: 13px; width: 100%; background: none; border: none; color: inherit; font-family: inherit; text-align: left; }
  .vpn-bar:hover { background: #131d2a; }
  .icon { font-size: 16px; }
  .label { font-weight: 600; }
  .status { flex: 1; text-align: right; font-size: 12px; color: #8a9aaa; }
  .status.on { color: #4fae4e; }
  .status.connecting { color: #ffaa00; }
  .status.err { color: #ff6b6b; }
  .tr { opacity: 0.75; font-weight: 400; }
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
  .pubkey-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; font-size: 11px; }
  .pubkey-row code { flex: 1; color: #cde; overflow: hidden; text-overflow: ellipsis; }
  .copy { background: none; border: 1px solid #333; border-radius: 6px; cursor: pointer; }
  .hint { font-size: 10px; color: #6a7a8a; margin: 0 0 10px; line-height: 1.4; }
  .hint.ok { color: #7aaf7a; }
  .hint code { font-size: 10px; }
  .err-box { background: #3a1a1a; color: #ff8a8a; font-size: 11px; padding: 8px; border-radius: 6px; margin-bottom: 8px; word-break: break-word; }
  .toggle-btn { width: 100%; padding: 10px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; background: #2a4a3a; color: #4fae4e; font-family: inherit; }
  .toggle-btn.on { background: #3a1a1a; color: #ff6b6b; }
  .toggle-btn:disabled { opacity: 0.6; cursor: wait; }
</style>
