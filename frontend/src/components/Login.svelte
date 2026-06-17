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
    background: #0a0a0a;
  }
  .login-card {
    background: rgba(15, 15, 15, 0.75);
    backdrop-filter: blur(30px);
    border: 1px solid rgba(255, 255, 255, 0.15);
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
    filter: drop-shadow(0 0 15px rgba(255, 82, 82, 0.5));
  }
  .login-title {
    font-size: 24px;
    font-weight: 700;
    color: #ffffff;
    margin-bottom: 8px;
  }
  .login-subtitle {
    font-size: 13px;
    color: rgba(255, 255, 255, 0.7);
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
    color: rgba(255, 255, 255, 0.6);
    text-transform: uppercase;
    letter-spacing: 0.8px;
    margin-bottom: 8px;
    font-weight: 600;
  }
  .form-group input {
    width: 100%;
    padding: 14px 16px;
    background: rgba(0, 0, 0, 0.5);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 14px;
    color: #ffffff;
    font-size: 15px;
    outline: none;
    transition: all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .form-group input:focus {
    border-color: #4fc3f7;
    background: rgba(255, 255, 255, 0.05);
    box-shadow: 0 0 0 4px rgba(79, 195, 247, 0.15);
  }
  .form-group input:disabled {
    opacity: 0.5;
  }
  .login-btn {
    width: 100%;
    padding: 16px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #ffffff;
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
    border-top-color: #ffffff;
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
    color: #ffffff;
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
    background: rgba(255, 255, 255, 0.05);
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
    color: #ffffff;
    border-color: rgba(255, 255, 255, 0.2);
    transform: translateY(-1px);
  }
  .test-login-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>
