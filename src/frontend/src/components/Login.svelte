<script>
  import { login, signup } from '../lib/api.js';
  import { showToast } from '../lib/stores.js';
  import { icons } from '../lib/icons.js';

  let { onSuccess } = $props();

  let mode = $state('login'); // 'login' | 'signup'
  let npub = $state('');
  let username = $state('');
  let loading = $state(false);
  let error = $state('');

  async function handleSubmit(e) {
    e.preventDefault();
    error = '';
    loading = true;

    try {
      if (!npub.trim()) throw new Error('npub обязателен');
      if (mode === 'login') {
        await login(npub.trim());
        showToast('✅ Вход выполнен');
      } else {
        if (!username.trim()) throw new Error('Имя пользователя обязательно');
        await signup(npub.trim(), username.trim());
        showToast('✅ Аккаунт создан');
      }
      if (onSuccess) onSuccess();
    } catch (err) {
      error = err.message || 'Ошибка сервера';
    } finally {
      loading = false;
    }
  }

  function toggleMode() {
    mode = mode === 'login' ? 'signup' : 'login';
    error = '';
  }

  async function loginAsTestUser(num) {
    error = '';
    loading = true;
    try {
      const testNpub = `npub1testsuperuser000000000000000000000000000000000000000000${num}`;
      try {
        await login(testNpub);
      } catch (err) {
        if (err.status === 401) {
          await signup(testNpub, `Смартфон ${num}`);
        } else {
          throw err;
        }
      }
      showToast(`✅ Вход: Смартфон ${num}`);
      if (onSuccess) onSuccess();
    } catch (err) {
      error = err.message || `Ошибка тестового входа ${num}`;
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-screen">
  <div class="login-card">
    <div class="login-logo">{@html icons.logo}</div>
    <h1 class="login-title">Proxi Messenger</h1>
    <p class="login-subtitle">E2E-протокол · P2P · Без цензуры</p>

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

      <div class="test-users-container">
        <button type="button" class="test-login-btn" onclick={() => loginAsTestUser(1)} disabled={loading}>📱 Смартфон 1</button>
        <button type="button" class="test-login-btn" onclick={() => loginAsTestUser(2)} disabled={loading}>📱 Смартфон 2</button>
        <button type="button" class="test-login-btn" onclick={() => loginAsTestUser(3)} disabled={loading}>📱 Смартфон 3</button>
      </div>
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
    background: transparent;
  }
  .login-card {
    background: var(--bg-panel);
    backdrop-filter: blur(30px);
    border: 1px solid var(--border-glass);
    border-radius: 24px;
    padding: 40px 32px;
    width: 100%;
    max-width: 400px;
    text-align: center;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  }
  .login-logo {
    font-size: 52px;
    margin-bottom: 16px;
    filter: drop-shadow(0 0 15px var(--accent));
  }
  .login-title {
    font-size: 24px;
    font-weight: 700;
    color: var(--text-primary);
    margin-bottom: 8px;
  }
  .login-subtitle {
    font-size: 13px;
    color: var(--text-secondary);
    margin-bottom: 30px;
    line-height: 1.5;
  }
  .login-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    margin-bottom: 24px;
  }
  .login-error {
    background: rgba(255, 82, 82, 0.1);
    border: 1px solid rgba(255, 82, 82, 0.3);
    border-radius: 12px;
    padding: 12px 16px;
    color: #ff5252;
    font-size: 13px;
    text-align: left;
    font-weight: 500;
  }
  .form-group {
    text-align: left;
  }
  .form-group label {
    display: block;
    font-size: 11px;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.8px;
    margin-bottom: 8px;
    font-weight: 600;
  }
  .form-group input {
    width: 100%;
    padding: 14px 16px;
    background: var(--bg-glass);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 14px;
    color: var(--text-primary);
    font-size: 15px;
    outline: none;
    transition: all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .form-group input:focus {
    border-color: var(--accent);
    background: var(--bg-glass);
    box-shadow: 0 0 0 4px var(--selection-bg);
  }
  .form-group input:disabled {
    opacity: 0.5;
  }
  .login-btn {
    width: 100%;
    padding: 16px;
    background: var(--accent);
    color: var(--text-primary);
    border: none;
    border-radius: 14px;
    cursor: pointer;
    font-size: 16px;
    font-weight: 600;
    transition: all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    box-shadow: 0 4px 15px rgba(30, 136, 229, 0.4);
  }
  .login-btn:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(30, 136, 229, 0.6);
  }
  .login-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .btn-spinner {
    width: 18px;
    height: 18px;
    border: 2px solid rgba(255,255,255,0.3);
    border-top-color: var(--text-primary);
    border-radius: 50%;
    animation: spin 0.6s cubic-bezier(0.5, 0, 0.5, 1) infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  .toggle-mode {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    font-size: 14px;
    cursor: pointer;
    padding: 8px;
    transition: color 0.2s;
  }
  .toggle-mode:hover {
    color: var(--text-primary);
  }
  .link {
    color: #4fc3f7;
    font-weight: 500;
  }
  .test-users-container {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 12px;
  }
  .test-login-btn {
    width: 100%;
    padding: 14px;
    background: var(--bg-glass);
    color: rgba(255, 255, 255, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 14px;
    cursor: pointer;
    font-size: 15px;
    font-weight: 500;
    transition: all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    margin-top: 4px;
  }
  .test-login-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.1);
    color: var(--text-primary);
    border-color: rgba(255, 255, 255, 0.2);
    transform: translateY(-1px);
  }
  .test-login-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
