<script lang="ts">
  import * as stores from '../stores/messenger'
  import { setE2EEnabled, loadE2EPref, toggleVPN } from '../lib/api-extended'
  import { updateProfile } from '../lib/api'
  import { getTheme, toggleTheme, onThemeChange, type Theme } from '../lib/theme'
  import { generateContactQR } from '../lib/qr'
  import { loadIdentityAsync, deleteIdentity } from '../lib/identity'
  import { initPush, getPushPermission, hasPushSubscription } from '../lib/push'

  let tab = $state<'profile' | 'contacts' | 'chats' | 'advanced'>('profile')
  let e2eOn = $state(true)
  let vpnOn = $state(false)
  let currentProfile = $state<stores.Profile>({ pubkey: '', name: '', about: '', avatar: '👤' })
  let contactList = $state<stores.Contact[]>([])
  let chatList = $state<stores.Chat[]>([])

  stores.profile.subscribe(v => { currentProfile = v })
  stores.contacts.subscribe(v => { contactList = v })
  stores.chats.subscribe(v => { chatList = v })

  let theme = $state<Theme>('dark')
  let identity = $state<{ npub: string; nsec: string } | null>(null)
  let seedVisible = $state(false)
  let showSeed = $state(false)
  let pushEnabled = $state(false)
  let killSwitch = $state(false)
  // P4: DM/VPN-signaling ходит только через встроенный relay — публичные
  // релеи сливают соцграф. Поле информационное (канон: local /nostr).
  let relayList = $state(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/nostr`)

  async function loadSeed() {
    identity = await loadIdentityAsync() as any
  }
  loadSeed()

  function toggleSeed() {
    seedVisible = !seedVisible
    showSeed = true
  }

  async function exportSeed() {
    if (!identity) await loadSeed()
    if (!identity) return
    const blob = new Blob([JSON.stringify(identity, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'proxi-identity-backup.json'
    a.click()
    URL.revokeObjectURL(url)
  }

  async function enablePush() {
    pushEnabled = await initPush()
  }

  function toggleKillSwitch() {
    killSwitch = !killSwitch
    try { localStorage.setItem('proxi-kill-switch', killSwitch ? '1' : '0') } catch {}
  }

  getTheme() // ensure initialized
  onThemeChange(t => { theme = t })

  function doToggleTheme() { toggleTheme() }
  let newName = $state('')
  let newAbout = $state('')
  let newAvatar = $state('👤')

  $effect(() => {
    if (currentProfile) {
      newName = currentProfile.name
      newAbout = currentProfile.about
      newAvatar = currentProfile.avatar
    }
  })

  const AVATARS = ['👤','👨','👩','🧑','🤖','🦊','🐱','🐸','🌟','🔥','💎','🎮','🎵','🌍','🚀','⚡','🦅','🐺','🐻','🐼']

  function save() {
    updateProfile(newName, newAbout, newAvatar)
    stores.profile.update(p => ({ ...p, name: newName, about: newAbout, avatar: newAvatar }))
    stores.saveChats()
    stores.saveContacts()
    stores.saveProfile()
  }

  function copyKey() {
    navigator.clipboard.writeText(currentProfile?.pubkey || '')
  }

  // S-004: Logout — clear token and reload
  function logout() {
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem('proxi_token')
      localStorage.removeItem('proxi_name_' + (currentProfile?.pubkey || ''))
    }
    location.reload()
  }

  // N-002: Sound toggle
  let soundEnabled = $state(true)
  if (typeof localStorage !== 'undefined') {
    soundEnabled = localStorage.getItem('proxi_sound') !== 'off'
  }
  function toggleSound() {
    soundEnabled = !soundEnabled
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('proxi_sound', soundEnabled ? 'on' : 'off')
    }
  }


  let qrDataUrl = $state('')
  let showQR = $state(false)

  async function doShowQR() {
    if (showQR) { showQR = false; return }
    qrDataUrl = await generateContactQR({ type: 'nostr', pubkey: currentProfile?.pubkey || '', name: currentProfile?.name || '' })
    showQR = true
  }

  function clearData() {
    if (confirm('Удалить все данные? Чаты, контакты, ключи?')) {
      localStorage.clear()
      location.reload()
    }
  }
</script>

<div class="settings">
  <div class="sh">
    <button class="back" onclick={() => stores.showSettings.set(false)}>←</button>
    <h3>Настройки</h3>
  </div>

  <div class="tabs">
    <button class:active={tab === 'profile'} onclick={() => tab = 'profile'}>👤</button>
    <button class:active={tab === 'contacts'} onclick={() => tab = 'contacts'}>👥</button>
    <button class:active={tab === 'chats'} onclick={() => tab = 'chats'}>💬</button>
    <button class:active={tab === 'advanced'} onclick={() => tab = 'advanced'}>⚙️</button>
  </div>

  {#if tab === 'profile'}
    <div class="sec">
      <div class="av-grid">{#each AVATARS as a}<button class="av" class:sel={newAvatar === a} onclick={() => newAvatar = a}>{a}</button>{/each}</div>
      <label>Имя</label>
      <input type="text" bind:value={newName} />
      <label>О себе</label>
      <textarea bind:value={newAbout} rows="2"></textarea>
      <label>Public Key</label>
      <div class="kr"><code>{currentProfile?.pubkey?.slice(0, 30)}...</code><button onclick={copyKey}>📋</button></div>
      <button class="save" onclick={save}>💾 Сохранить</button>
      <button class="qr-btn" onclick={doShowQR}>📷 QR код</button>
      {#if showQR && qrDataUrl}
        <div class="qr-box"><img src={qrDataUrl} alt="QR" /><p>Покажите другу для добавления</p></div>
      {/if}
    </div>
  {:else if tab === 'advanced'}
    <div class="sec">
      <h4>🔐 Шифрование</h4>
      <div class="row row-between">
        <span>E2E шифрование</span>
        <button class="toggle-btn" class:on={e2eOn}
          onclick={() => { e2eOn = !e2eOn; setE2EEnabled(e2eOn); }}>
          {e2eOn ? 'ON ✅' : 'OFF ❌'}
        </button>
      </div>
      <p class="hint">XChaCha20-Poly1305 AEAD шифрование сообщений</p>
    </div>
    <div class="sec">
      <h4>🌐 VPN</h4>
      <div class="row row-between">
        <span>WebRTC Proxy</span>
        <button class="toggle-btn" class:on={vpnOn}
          onclick={async () => { vpnOn = !vpnOn; await toggleVPN(vpnOn); }}>
          {vpnOn ? 'ON ✅' : 'OFF ❌'}
        </button>
      </div>
      <p class="hint">Маршрутизация трафика через peer SOCKS5</p>
    </div>
  {:else if tab === 'contacts'}
    <div class="sec">
      <h4>Контакты ({contactList.length})</h4>
      {#each contactList as c}
        <div class="ci"><span class="ca">{c.avatar}</span><span class="cn">{c.name}</span><span class="cs">{c.isOnline ? '🟢' : '⚫'}</span></div>
      {/each}
      {#if contactList.length === 0}<p class="emp">Пусто</p>{/if}
    </div>
  {:else if tab === 'chats'}
    <div class="sec">
      <h4>Чаты ({chatList.length})</h4>
      {#each chatList as c}
        <div class="ci"><span class="ca">{c.avatar}</span><div><span class="cn">{c.name}</span><br><span class="cp">{c.id.slice(0, 20)}...</span></div></div>
      {/each}
    </div>
  {:else if tab === 'advanced'}

    <div class="setting-row">
      <span>🔊 Звук уведомлений</span>
      <button onclick={toggleSound}>{soundEnabled ? '✅ Вкл' : '❌ Выкл'}</button>
    </div>
    <div class="setting-row">
      <button class="logout-btn" onclick={logout}>🚪 Выйти</button>
    </div>
    <div class="sec">
      <h4>Оформление</h4>
      <button class="theme-btn" onclick={doToggleTheme}>
        {theme === 'dark' ? '☀️ Светлая тема' : '🌙 Тёмная тема'}
      </button>
      <h4>Данные</h4>
      <button class="danger" onclick={clearData}>🗑️ Удалить все данные</button>
      <p class="hint">Это удалит ключи, чаты, контакты. Без восстановления.</p>
    </div>
  {/if}

    <!-- ONB-002: Seed backup -->
    <div class="settings-section" data-testid="settings-seed">
      <h4>🔑 Аккаунт и ключи</h4>
      <div class="btn-row">
        <button data-testid="settings-show-seed" onclick={toggleSeed}>
          {showSeed ? '👁️ Скрыть ключи' : '🔑 Показать nsec'}
        </button>
        <button data-testid="settings-export-seed" onclick={exportSeed}>💾 Экспорт JSON</button>
      </div>
      {#if showSeed && identity}
        <div class="seed-box">
          <label>npub:</label>
          <div data-testid="settings-npub">{identity.npub}</div>
          {#if seedVisible}
            <label class="warn-label">nsec (никому не показывайте):</label>
            <div data-testid="settings-nsec" class="nsec">{identity.nsec}</div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- ONB-002: Push notifications -->
    <div class="settings-section">
      <h4>🔔 Уведомления</h4>
      <button data-testid="settings-push-toggle" onclick={enablePush}>
        {pushEnabled ? '✅ Push включены' : 'Включить push'}
      </button>
    </div>

    <!-- ONB-002: Kill switch -->
    <div class="settings-section">
      <h4>🛡️ VPN Kill Switch</h4>
      <button data-testid="settings-kill-switch" onclick={toggleKillSwitch}>
        {killSwitch ? '✅ Вкл (блокировать без VPN)' : '❌ Выкл'}
      </button>
    </div>

    <!-- ONB-002: Relay list -->
    <div class="settings-section">
      <h4>📡 Nostr Relay</h4>
      <textarea data-testid="settings-relays" bind:value={relayList} rows="4" class="relay-ta"></textarea>
    </div>

  </div>

<style>
  .settings { width: 360px; background: var(--bg-secondary); height: 100vh; overflow-y: auto; border-right: 1px solid var(--border); }
  .sh { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
  h3,h4 { margin: 0; } h4 { color: var(--text-muted); font-size: 13px; margin: 16px 0 8px; }
  .back { background: none; border: none; color: var(--text-muted); font-size: 18px; cursor: pointer; width: auto; padding: 4px 8px; }
  .tabs { display: flex; border-bottom: 1px solid var(--border); }
  .tabs button { flex: 1; background: none; border: none; color: var(--text-muted); padding: 12px; cursor: pointer; font-size: 16px; }
  .tabs button.active { color: var(--accent); border-bottom: 2px solid var(--accent); }
  .sec { padding: 16px; }
  label { display: block; font-size: 12px; color: var(--text-muted); margin: 8px 0 4px; }
  input,textarea { width: 100%; background: var(--bg-tertiary); border: none; color: var(--text); padding: 10px; border-radius: 8px; font-size: 13px; margin-bottom: 8px; box-sizing: border-box; font-family: inherit; }
  .av-grid { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
  .av { width: 36px; height: 36px; border-radius: 50%; background: var(--bg-tertiary); border: 2px solid transparent; font-size: 18px; cursor: pointer; display: flex; align-items: center; justify-content: center; padding: 0; }
  .av.sel { border-color: var(--accent); }
  .kr { display: flex; gap: 6px; align-items: center; }
  .kr code { flex: 1; background: var(--bg-tertiary); padding: 8px; border-radius: 6px; font-size: 12px; word-break: break-all; }
  .kr button { background: var(--bg-tertiary); border: none; color: var(--text-muted); padding: 6px 10px; border-radius: 6px; cursor: pointer; width: auto; }
  .save { width: 100%; padding: 12px; background: var(--accent); border: none; color: var(--text-on-accent); border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
  .danger { width: 100%; padding: 10px; background: color-mix(in srgb, var(--danger) 22%, var(--bg)); border: none; color: var(--danger); border-radius: 8px; cursor: pointer; }
  .theme-btn { width: 100%; padding: 10px; background: var(--bg-tertiary); border: none; color: var(--text); border-radius: 8px; cursor: pointer; font-size: 14px; font-family: inherit; }
  .hint { font-size: 12px; color: var(--text-muted); margin-top: 6px; }
  .ci { display: flex; align-items: center; gap: 10px; padding: 8px; border-radius: 8px; }
  .ca { font-size: 20px; width: 36px; height: 36px; background: color-mix(in srgb, var(--success) 22%, var(--bg)); border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .cn { font-size: 14px; } .cs { font-size: 12px; } .cp { font-size: 12px; color: var(--text-muted); }
  .emp { color: var(--text-muted); text-align: center; padding: 20px; font-size: 13px; }
  .qr-btn { width: 100%; padding: 10px; background: var(--bg-tertiary); border: none; color: var(--accent); border-radius: 8px; cursor: pointer; font-size: 14px; margin-top: 8px; font-family: inherit; }
  .qr-box { text-align: center; margin-top: 12px; }
  .qr-box img { width: 200px; height: 200px; border-radius: 12px; }
  .qr-box p { font-size: 12px; color: var(--text-muted); margin-top: 8px; }

  .row-between { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; }
  .toggle-btn { background: var(--bg-tertiary); color: var(--text-on-accent); border: none; padding: 8px 16px; border-radius: 8px; cursor: pointer; transition: all 180ms ease-in-out; }
  .toggle-btn.on { background: var(--accent); }
  .toggle-btn:hover:not(:disabled) { filter: brightness(1.08); }
  .toggle-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .btn-row { display: flex; gap: 8px; flex-wrap: wrap; }
  .seed-box { margin-top: 8px; font-family: monospace; font-size: 12px; word-break: break-all; background: var(--bg); padding: 8px; border-radius: 8px; }
  .warn-label { color: var(--warn); margin-top: 8px; display: block; }
  .nsec { color: var(--warn); }
  .relay-ta { width: 100%; background: var(--bg-secondary); color: var(--text); border: 1px solid var(--border); border-radius: 8px; padding: 8px; font-family: monospace; font-size: 12px; }
  .save:hover:not(:disabled), .danger:hover:not(:disabled), .theme-btn:hover:not(:disabled), .qr-btn:hover:not(:disabled), .back:hover { filter: brightness(1.08); }
  .save:focus-visible, .danger:focus-visible, .theme-btn:focus-visible, .qr-btn:focus-visible, .back:focus-visible, .tabs button:focus-visible {
    outline: 2px solid var(--accent); outline-offset: 2px;
  }
  button, input, textarea { transition: all 180ms ease-in-out; }
  .hint { font-size: 12px; color: var(--text-muted); margin-top: 4px; }
  .cs, .cp { font-size: 12px; }
</style>
