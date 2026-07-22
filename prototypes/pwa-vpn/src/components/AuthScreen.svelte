<!--
  T92: Authentication / Connection screen.
  Nostr key generation + login.
-->
<script lang="ts">
  import { authenticate } from '../lib/effector';
  import { authApi } from '../lib/api';

  let mode = 'generate'; // 'generate' | 'import'
  let privkey = '';
  let pubkey = '';
  let loading = false;
  let error = '';

  async function generateKey() {
    loading = true;
    try {
      // Generate Ed25519 keypair via WebCrypto
      const keyPair = await crypto.subtle.generateKey('Ed25519', true, ['sign', 'verify']);
      const rawPriv = await crypto.subtle.exportKey('pkcs8', keyPair.privateKey);
      const rawPub = await crypto.subtle.exportKey('raw', keyPair.publicKey);
      privkey = btoa(String.fromCharCode(...new Uint8Array(rawPriv)));
      pubkey = btoa(String.fromCharCode(...new Uint8Array(rawPub)));
      mode = 'show';
    } catch (e) {
      error = `Key generation failed: ${e}`;
    }
    loading = false;
  }

  async function login() {
    loading = true;
    try {
      authenticate({ pubkey, name: 'User' });
    } catch (e) {
      error = `Login failed: ${e}`;
    }
    loading = false;
  }
</script>

<div class="auth-screen">
  <h1>🔐 Proxi</h1>
  <p>Неубиваемый мессенджер</p>

  {#if mode === 'generate' && !privkey}
    <button on:click={generateKey} disabled={loading}>
      {loading ? 'Генерация...' : 'Создать новый аккаунт'}
    </button>
    <button on:click={() => mode = 'import'}>Импортировать ключ</button>
  {:else if mode === 'show'}
    <p>Ваш публичный ключ:</p>
    <code>{pubkey}</code>
    <p>Сохраните приватный ключ в безопасном месте!</p>
    <code class="privkey">{privkey}</code>
    <button on:click={login}>Войти →</button>
  {:else}
    <input bind:value={privkey} placeholder="Приватный ключ" type="password" />
    <button on:click={login} disabled={loading || !privkey}>
      {loading ? 'Вход...' : 'Войти'}
    </button>
  {/if}

  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .auth-screen { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 48px; }
  h1 { font-size: 2rem; }
  code { word-break: break-all; font-size: 0.8rem; background: #f0f0f0; padding: 8px; border-radius: 4px; }
  .privkey { color: #d32f2f; }
  .error { color: #d32f2f; }
  button { padding: 12px 24px; border: none; border-radius: 8px; background: #6200ee; color: white; cursor: pointer; }
  button:disabled { opacity: 0.5; }
</style>
