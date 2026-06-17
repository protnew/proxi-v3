<script>
  import { login, signup } from '../lib/api.js';
  import { showToast } from '../lib/stores.js';

  let { onSuccess } = $props();

  let mode = $state('login'); // 'login' | 'signup'
  let npub = $state('');
  let username = $state('');
  let loading = $state(false);
  let error = $state('');

  async function handleSubmit(e) {
    e.preventDefault();
    error = '';

    const npubVal = npub.trim();
    if (!npubVal) {
      error = 'Введите npub';
      return;
    }
    if (npubVal.length < 10) {
      error = 'npub слишком короткий';
      return;
    }

    loading = true;
    try {
      if (mode === 'signup') {
        const user = username.trim();
        if (!user) {
          error = 'Введите имя пользователя';
          loading = false;
          return;
        }
        await signup(npubVal, user);
        showToast('✅ Аккаунт создан!');
      } else {
        await login(npubVal);
        showToast('✅ Вход выполнен!');
      }
      if (onSuccess) onSuccess();
    } catch (err) {
      if (err.status === 401) {
        error = 'Неверный npub или не зарегистрирован';
      } else {
        error = err.message || 'Ошибка подключения к серверу';
      }
    } finally {
      loading = false;
    }
  }

  function toggleMode() {
    mode = mode === 'login' ? 'signup' : 'login';
    error = '';
  }

  async function handleTestLogin() {
    error = '';
    loading = true;
    try {
      const testNpub = "npub1testsuperuser0000000000000000000000000000000000000000000";
      try {
        await login(testNpub);
      } catch (err) {
        if (err.status === 401) {
          await signup(testNpub, "TestUser");
        } else {
          throw err;
        }
      }
      showToast('✅ Вход под тестовым пользователем!');
      if (onSuccess) onSuccess();
    } catch (err) {
      error = err.message || 'Ошибка тестового входа';
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-screen">
  <div class="login-card">
    <div class="login-logo">🔥</div>
    <h1 class="login-title">Proxi Messenger</h1>
    <p class="login-subtitle">Nostr-протокол · P2P · Без цензуры</p>

    <form onsubmit={handleSubmit} class="login-form">
      {#if error}
        <div class="login-error">{error}</div>
      {/if}

      <div class="form-group">
        <label for="npub-input">npub</label>
        <input
          id="npub-input"
          type="text"
          bind:value={npub}
          placeholder="npub1..."
          disabled={loading}
          autocomplete="username"
        />
      </div>

      {#if mode === 'signup'}
        <div class="form-group">
          <label for="username-input">Имя пользователя</label>
          <input
            id="username-input"
            type="text"
            bind:value={username}
            placeholder="Ваше имя"
            disabled={loading}
            autocomplete="nickname"
          />
        </div>
      {/if}

      <button type="submit" class="login-btn" disabled={loading}>
        {#if loading}
          <span class="btn-spinner"></span>
          {mode === 'login' ? 'Вход...' : 'Регистрация...'}
        {:else}
          {mode === 'login' ? '🔑 Войти' : '🚀 Создать аккаунт'}
        {/if}
      </button>

      <button type="button" class="test-login-btn" onclick={handleTestLogin} disabled={loading}>
        🧪 Войти как тестовый пользователь
      </button>
    </form>

    <button class="toggle-mode" onclick={toggleMode}>
      {#if mode === 'login'}
        Нет аккаунта? <span class="link">Зарегистрироваться</span>
      {:else}
        Уже есть аккаунт? <span class="link">Войти</span>
      {/if}
    </button>
  </div>
</div>

<style>
  .login-screen {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
    background: #0a0a0a;
  }
  .login-card {
    background: #111;
    border: 1px solid #222;
    border-radius: 20px;
    padding: 40px 32px;
    width: 100%;
    max-width: 380px;
    text-align: center;
  }
  .login-logo {
    font-size: 48px;
    margin-bottom: 12px;
  }
  .login-title {
    font-size: 22px;
    font-weight: 600;
    color: #e0e0e0;
    margin-bottom: 6px;
  }
  .login-subtitle {
    font-size: 12px;
    color: #555;
    margin-bottom: 28px;
    line-height: 1.5;
  }
  .login-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
    margin-bottom: 20px;
  }
  .login-error {
    background: #1a0a0a;
    border: 1px solid #c62828;
    border-radius: 8px;
    padding: 10px 14px;
    color: #ef5350;
    font-size: 13px;
    text-align: left;
  }
  .form-group {
    text-align: left;
  }
  .form-group label {
    display: block;
    font-size: 11px;
    color: #666;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 6px;
  }
  .form-group input {
    width: 100%;
    padding: 12px 14px;
    background: #0a0a0a;
    border: 1px solid #333;
    border-radius: 10px;
    color: #e0e0e0;
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
  }
  .form-group input:focus {
    border-color: #1e88e5;
  }
  .form-group input:disabled {
    opacity: 0.5;
  }
  .login-btn {
    width: 100%;
    padding: 14px;
    background: #1e88e5;
    color: #fff;
    border: none;
    border-radius: 12px;
    cursor: pointer;
    font-size: 15px;
    font-weight: 500;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }
  .login-btn:hover:not(:disabled) {
    background: #1565c0;
  }
  .login-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .btn-spinner {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255,255,255,0.3);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  .toggle-mode {
    background: none;
    border: none;
    color: #555;
    font-size: 13px;
    cursor: pointer;
    padding: 8px;
  }
  .toggle-mode:hover {
    color: #888;
  }
  .link {
    color: #4fc3f7;
  }
  .test-login-btn {
    width: 100%;
    padding: 12px;
    background: #2a2a2a;
    color: #a0a0a0;
    border: 1px solid #444;
    border-radius: 12px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    margin-top: 10px;
  }
  .test-login-btn:hover:not(:disabled) {
    background: #333;
    color: #fff;
    border-color: #666;
  }
  .test-login-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
