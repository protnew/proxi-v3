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
</script>

<div class="auth-screen" data-testid="auth-screen" role="main" aria-label="Создание аккаунта">
  <h1>Proxi</h1>
  <p class="sub">Неубиваемый мессенджер · ключи только на вашем устройстве</p>

  {#if mode === 'start'}
    <button class="primary" data-testid="auth-create" onclick={generate} disabled={loading} aria-label="Создать новый аккаунт">
      {loading ? 'Генерация…' : 'Создать новый аккаунт'}
    </button>
    <button class="secondary" data-testid="auth-import-open" onclick={() => mode = 'import'}>Импортировать nsec / seed</button>
  {:else if mode === 'show' && identity}
    <p class="warn">⚠️ Сохраните nsec. Без него аккаунт не восстановить.</p>
    <label>npub
      <input data-testid="auth-npub" readonly value={identity.npub || identity.publicKey} />
    </label>
    <label>nsec
      <input data-testid="auth-nsec" readonly value={identity.nsec || identity.privateKey} />
    </label>
    <label class="check">
      <input type="checkbox" data-testid="auth-backup-ok" bind:checked={confirmedBackup} />
      Я сохранил nsec в надёжном месте
    </label>
    <button class="primary" data-testid="auth-enter" onclick={enter} disabled={!confirmedBackup}>Войти</button>
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
    <input data-testid="auth-npub" readonly value={identity.npub || identity.publicKey} />
    <button class="primary" data-testid="auth-enter" onclick={enter}>Продолжить</button>
  {/if}

  {#if error}
    <p class="err" data-testid="auth-error">{error}</p>
  {/if}
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
    background: #0f172a;
    color: #f8fafc;
  }
  h1 { margin: 0; font-size: 2rem; }
  .sub { color: #94a3b8; margin: 0 0 12px; text-align: center; }
  label { display: flex; flex-direction: column; gap: 6px; width: min(420px, 100%); font-size: 13px; color: #94a3b8; }
  input[type="text"], input:not([type]), input[readonly] {
    padding: 10px 12px; border-radius: 8px; border: 1px solid #334155; background: #1e293b; color: #f8fafc;
  }
  .primary, .secondary {
    width: min(420px, 100%); padding: 12px; border-radius: 10px; border: none; cursor: pointer; font-weight: 600;
  }
  .primary { background: #3b82f6; color: white; }
  .primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .secondary { background: #1e293b; color: #93c5fd; border: 1px solid #334155; }
  .warn { color: #fbbf24; max-width: 420px; text-align: center; }
  .ok { color: #34d399; }
  .err { color: #f87171; }
  .check { flex-direction: row; align-items: center; gap: 8px; color: #e2e8f0; }
</style>
