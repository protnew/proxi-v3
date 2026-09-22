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
  let relayList = $state('wss://relay.damus.io\nwss://nos.lol\nwss://relay.nostr.band')

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
      <div class="row" style="display:flex;justify-content:space-between;align-items:center;padding:10px 0">
        <span>E2E шифрование</span>
        <button style="background:{e2eOn ? '#3b82f6' : '#333'};color:white;border:none;padding:6px 16px;border-radius:8px;cursor:pointer"
          onclick={() => { e2eOn = !e2eOn; setE2EEnabled(e2eOn); }}>
          {e2eOn ? 'ON ✅' : 'OFF ❌'}
        </button>
      </div>
      <p style="font-size:11px;color:#666;margin-top:4px">XChaCha20-Poly1305 AEAD шифрование сообщений</p>
    </div>
    <div class="sec">
      <h4>🌐 VPN</h4>
      <div class="row" style="display:flex;justify-content:space-between;align-items:center;padding:10px 0">
        <span>WebRTC Proxy</span>
        <button style="background:{vpnOn ? '#3b82f6' : '#333'};color:white;border:none;padding:6px 16px;border-radius:8px;cursor:pointer"
          onclick={async () => { vpnOn = !vpnOn; await toggleVPN(vpnOn); }}>
          {vpnOn ? 'ON ✅' : 'OFF ❌'}
        </button>
      </div>
      <p style="font-size:11px;color:#666;margin-top:4px">Маршрутизация трафика через peer SOCKS5</p>
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
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        <button data-testid="settings-show-seed" onclick={toggleSeed}>
          {showSeed ? '👁️ Скрыть ключи' : '🔑 Показать nsec'}
        </button>
        <button data-testid="settings-export-seed" onclick={exportSeed}>💾 Экспорт JSON</button>
      </div>
      {#if showSeed && identity}
        <div style="margin-top:8px;font-family:monospace;font-size:11px;word-break:break-all;background:#0f172a;padding:8px;border-radius:6px">
          <label style="color:#94a3b8">npub:</label>
          <div data-testid="settings-npub">{identity.npub}</div>
          {#if seedVisible}
            <label style="color:#fbbf24;margin-top:8px;display:block">⚠️ nsec (никому не показывайте):</label>
            <div data-testid="settings-nsec" style="color:#fbbf24">{identity.nsec}</div>
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
      <textarea data-testid="settings-relays" bind:value={relayList} rows="4" style="width:100%;background:#1e293b;color:#e2e8f0;border:1px solid #334155;border-radius:6px;padding:8px;font-family:monospace;font-size:12px"></textarea>
    </div>

  </div>

<style>
  .settings { width: 360px; background: #17212b; height: 100vh; overflow-y: auto; border-right: 1px solid #0e1621; }
  .sh { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid #0e1621; }
  h3,h4 { margin: 0; } h4 { color: #7a8a9a; font-size: 13px; margin: 16px 0 8px; }
  .back { background: none; border: none; color: #aaa; font-size: 18px; cursor: pointer; width: auto; padding: 4px 8px; }
  .tabs { display: flex; border-bottom: 1px solid #0e1621; }
  .tabs button { flex: 1; background: none; border: none; color: #7a8a9a; padding: 12px; cursor: pointer; font-size: 16px; }
  .tabs button.active { color: #3a9aff; border-bottom: 2px solid #3a9aff; }
  .sec { padding: 16px; }
  label { display: block; font-size: 12px; color: #7a8a9a; margin: 8px 0 4px; }
  input,textarea { width: 100%; background: #242f3d; border: none; color: #e0e0e0; padding: 10px; border-radius: 8px; font-size: 13px; margin-bottom: 8px; box-sizing: border-box; font-family: inherit; }
  .av-grid { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
  .av { width: 36px; height: 36px; border-radius: 50%; background: #242f3d; border: 2px solid transparent; font-size: 18px; cursor: pointer; display: flex; align-items: center; justify-content: center; padding: 0; }
  .av.sel { border-color: #3a9aff; }
  .kr { display: flex; gap: 6px; align-items: center; }
  .kr code { flex: 1; background: #242f3d; padding: 8px; border-radius: 6px; font-size: 11px; word-break: break-all; }
  .kr button { background: #242f3d; border: none; color: #aaa; padding: 6px 10px; border-radius: 6px; cursor: pointer; width: auto; }
  .save { width: 100%; padding: 12px; background: #3a7bd5; border: none; color: white; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
  .danger { width: 100%; padding: 10px; background: #3a1a1a; border: none; color: #ff6b6b; border-radius: 8px; cursor: pointer; }
  .theme-btn { width: 100%; padding: 10px; background: #242f3d; border: none; color: #e0e0e0; border-radius: 8px; cursor: pointer; font-size: 14px; font-family: inherit; }
  .hint { font-size: 11px; color: #555; margin-top: 6px; }
  .ci { display: flex; align-items: center; gap: 10px; padding: 8px; border-radius: 8px; }
  .ca { font-size: 20px; width: 36px; height: 36px; background: #3a5a3a; border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .cn { font-size: 14px; } .cs { font-size: 10px; } .cp { font-size: 10px; color: #555; }
  .emp { color: #555; text-align: center; padding: 20px; font-size: 13px; }
  .qr-btn { width: 100%; padding: 10px; background: #242f3d; border: none; color: #3a9aff; border-radius: 8px; cursor: pointer; font-size: 14px; margin-top: 8px; font-family: inherit; }
  .qr-box { text-align: center; margin-top: 12px; }
  .qr-box img { width: 200px; height: 200px; border-radius: 12px; }
  .qr-box p { font-size: 11px; color: #7a8a9a; margin-top: 8px; }
</style>
