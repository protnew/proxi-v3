<!-- ONB-000: Identity genesis — real secp256k1 npub/nsec via identity.ts -->
<script lang="ts">
  import { createIdentity, importIdentity, loadIdentityAsync, type Identity } from '../lib/identity'

  let { onDone }: { onDone?: () => void } = $props()

  let mode = $state<'start' | 'show' | 'import' | 'ready'>('start')
  let identity = $state<Identity | null>(null)
  let importKey = $state('')
  let loading = $state(false)
  let error = $state('')
  let confirmedBackup = $state(false)
  let nsecVisible = $state(false)

  $effect(() => {
    loadIdentityAsync().then(id => {
      if (id) {
        identity = id
        mode = 'ready'
      }
    }).catch(() => {})
  })

  async function generate() {
    loading = true
    error = ''
    try {
      identity = await createIdentity()
      mode = 'show'
      confirmedBackup = false
    } catch (e) {
      error = `Генерация не удалась: ${e}`
    }
    loading = false
  }

  async function doImport() {
    loading = true
    error = ''
    try {
      identity = await importIdentity(importKey.trim())
      mode = 'ready'
    } catch (e) {
      error = `Импорт не удался: ${e}`
    }
    loading = false
  }

  function enter() {
    if (!identity) return
    if (mode === 'show' && !confirmedBackup) {
      error = 'Подтвердите, что сохранили seed/nsec'
      return
    }
    onDone?.()
  }

  async function copy(text: string) {
    try { await navigator.clipboard.writeText(text) } catch {}
  }
</script>

<div class="auth-screen" data-testid="auth-screen" role="main" aria-label="Создание аккаунта">
  <div class="auth-card">
  <h1>Proxi</h1>
  <p class="sub">Неубиваемый мессенджер · ключи только на вашем устройстве</p>

  {#if mode === 'start'}
    <button class="primary" data-testid="auth-create" onclick={generate} disabled={loading} aria-label="Создать новый аккаунт">
      {loading ? 'Генерация…' : 'Создать новый аккаунт'}
    </button>
    <button class="secondary" data-testid="auth-import-open" onclick={() => mode = 'import'}>Импортировать nsec / seed</button>
  {:else if mode === 'show' && identity}
    <p class="warn">Сохраните nsec. Без него аккаунт не восстановить.</p>
    <label>npub
      <div class="key-row">
        <input data-testid="auth-npub" readonly value={identity.npub || identity.publicKey} />
        <button type="button" class="icon-btn" onclick={() => copy(identity?.npub || identity?.publicKey || '')}>копировать</button>
      </div>
    </label>
    <label>nsec
      <div class="key-row">
        <input data-testid="auth-nsec" readonly type={nsecVisible ? 'text' : 'password'} value={identity.nsec || identity.privateKey} />
        <button type="button" class="icon-btn" onclick={() => nsecVisible = !nsecVisible}>{nsecVisible ? 'скрыть' : 'показать'}</button>
        <button type="button" class="icon-btn" onclick={() => copy(identity?.nsec || identity?.privateKey || '')}>копировать</button>
      </div>
    </label>
    <label class="check">
      <input type="checkbox" data-testid="auth-backup-ok" bind:checked={confirmedBackup} />
      Я сохранил nsec в надёжном месте
    </label>
    <button class="primary" data-testid="auth-enter" onclick={enter} disabled={!confirmedBackup} aria-disabled={!confirmedBackup} aria-describedby="auth-enter-hint">Войти</button>
    {#if !confirmedBackup}
      <p id="auth-enter-hint" class="hint">Войти станет доступно после галки выше</p>
    {/if}
  {:else if mode === 'import'}
    <label>nsec или hex private key
      <input data-testid="auth-import-input" bind:value={importKey} placeholder="nsec1… или 64 hex" />
    </label>
    <button class="primary" data-testid="auth-import" onclick={doImport} disabled={loading || importKey.trim().length < 16}>
      {loading ? 'Импорт…' : 'Импортировать'}
    </button>
    <button class="secondary" onclick={() => mode = 'start'}>Назад</button>
  {:else if mode === 'ready' && identity}
    <p class="ok">Аккаунт найден</p>
    <input class="key-full" data-testid="auth-npub" readonly value={identity.npub || identity.publicKey} />
    <button class="primary" data-testid="auth-enter" onclick={enter}>Продолжить</button>
  {/if}

  {#if error}
    <p class="err" data-testid="auth-error">{error}</p>
  {/if}
  </div>
</div>

<style>
  .auth-screen {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 24px;
    background: var(--bg);
    color: var(--text);
  }
  .auth-card {
    min-height: 420px;
    width: min(420px, 100%);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 32px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    box-shadow: var(--shadow);
  }
  h1 { margin: 0; font-size: 32px; }
  .sub { color: var(--text-muted); margin: 0 0 12px; text-align: center; }
  label { display: flex; flex-direction: column; gap: 8px; width: 100%; font-size: 12px; color: var(--text-muted); }
  input[type="text"], input[type="password"], input:not([type]), input[readonly] {
    padding: 12px 16px; border-radius: 8px; border: 1px solid var(--border);
    background: var(--bg-tertiary); color: var(--text);
    overflow-wrap: anywhere; word-break: break-all; font-size: 12px; width: 100%;
  }
  input:focus-visible {
    outline: 2px solid var(--accent); outline-offset: 2px;
  }
  .key-row { display: flex; gap: 8px; align-items: stretch; width: 100%; }
  .key-row input { flex: 1; min-width: 0; }
  .key-full { width: 100%; overflow-wrap: anywhere; word-break: break-all; }
  .icon-btn {
    background: var(--bg-tertiary); color: var(--text); border: 1px solid var(--border);
    border-radius: 8px; padding: 8px 12px; cursor: pointer; font-size: 12px; white-space: nowrap;
    transition: all 180ms ease-in-out;
  }
  .icon-btn:hover { background: var(--bg-hover); }
  .icon-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .primary, .secondary {
    width: 100%; padding: 12px 16px; border-radius: 8px; border: none; cursor: pointer; font-weight: 600;
    transition: all 180ms ease-in-out;
  }
  .primary { background: var(--accent); color: var(--text-on-accent); }
  .primary:hover:not(:disabled) { filter: brightness(1.08); }
  .primary:focus-visible, .secondary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .primary:disabled { opacity: 1; background: var(--bg-tertiary); color: var(--text-muted); cursor: not-allowed; }
  .secondary { background: var(--bg-secondary); color: var(--text); border: 1px solid var(--border); }
  .secondary:hover:not(:disabled) { background: var(--bg-hover); }
  .warn { color: var(--warn); max-width: 100%; text-align: center; margin: 0; }
  .ok { color: var(--success); }
  .err { color: var(--danger); }
  .hint { color: var(--text-muted); font-size: 12px; margin: 0; text-align: center; }
  .check { flex-direction: row; align-items: center; gap: 8px; color: var(--text); }
  .check input[type="checkbox"] {
    width: 16px; height: 16px; accent-color: var(--accent); flex-shrink: 0;
  }
</style>
