<script>
  import { onMount } from 'svelte';
  import { showToast } from '../lib/stores.js';
  import { apiFetch } from '../lib/api.js';

  let switchActive = $state(false);
  let messageText = $state('');
  let intervalDays = $state(7);
  let recipient = $state('broadcast');
  let loading = $state(false);

  // Load existing switch config
  onMount(async () => {
    loading = true;
    try {
      const res = await apiFetch('/api/switch');
      if (res && res.id) {
        switchActive = true;
        messageText = res.message_text || '';
        intervalDays = res.interval_days || 7;
        recipient = res.recipient || 'broadcast';
      }
    } catch (e) {
      console.log('No active switch found or error fetching config.');
    }
    loading = false;
  });

  async function saveConfig() {
    loading = true;
    try {
      if (switchActive) {
        if (!messageText.trim()) {
          showToast('❌ Введите текст сообщения');
          loading = false;
          return;
        }
        await apiFetch('/api/switch', {
          method: 'POST',
          body: JSON.stringify({
            message_text: messageText,
            interval_days: parseInt(intervalDays, 10),
            recipient: recipient
          })
        });
        showToast('✅ Конфигурация обновлена');
      } else {
        await apiFetch('/api/switch', {
          method: 'DELETE'
        });
        showToast('✅ Dead Man\'s Switch отключён');
      }
    } catch (e) {
      showToast('❌ Ошибка сохранения');
      console.error(e);
    }
    loading = false;
  }

  async function checkIn() {
    loading = true;
    try {
      await apiFetch('/api/switch/checkin', { method: 'POST' });
      showToast('🔄 Отметка об активности отправлена');
    } catch (e) {
      showToast('❌ Ошибка отметки');
      console.error(e);
    }
    loading = false;
  }
</script>

<div class="panel glass-panel">
  <div class="header">
    <div class="icon-wrap">💀</div>
    <h2>Dead Man's Switch</h2>
  </div>

  <div class="content">
    <p class="description">
      Если вы не заходите в приложение в течение указанного времени, система автоматически отправит заданное сообщение выбранному адресату (или всем).
    </p>

    {#if loading}
      <div class="loading">Загрузка...</div>
    {:else}
      <div class="form-group row">
        <label for="switchToggle">Активировать</label>
        <label class="switch">
          <input id="switchToggle" type="checkbox" bind:checked={switchActive} onchange={saveConfig}>
          <span class="slider round"></span>
        </label>
      </div>

      {#if switchActive}
        <div class="form-group">
          <label for="messageText">Сообщение</label>
          <textarea 
            id="messageText" 
            bind:value={messageText} 
            placeholder="Сообщение, которое будет отправлено..." 
            rows="4"
          ></textarea>
        </div>

        <div class="form-group">
          <label for="intervalDays">Дней до отправки ({intervalDays})</label>
          <input 
            id="intervalDays" 
            type="range" 
            min="1" 
            max="365" 
            bind:value={intervalDays} 
          />
        </div>

        <div class="form-group">
          <label for="recipient">Получатель</label>
          <select id="recipient" bind:value={recipient}>
            <option value="broadcast">Всем (Широковещательно)</option>
            <!-- Here we could dynamically load contacts or channels -->
            <option value="contacts">Только контактам</option>
          </select>
        </div>

        <div class="actions">
          <button class="btn primary" onclick={saveConfig}>💾 Сохранить</button>
          <button class="btn secondary outline" onclick={checkIn}>🔄 Я жив (Отметиться)</button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
    padding: 20px;
    box-sizing: border-box;
    border-radius: var(--border-radius-lg);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 15px;
    margin-bottom: 24px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border-color);
  }

  .icon-wrap {
    font-size: 2rem;
    background: var(--bg-glass);
    padding: 10px;
    border-radius: 50%;
    box-shadow: var(--shadow-sm);
  }

  h2 {
    margin: 0;
    color: var(--text-primary);
    font-weight: 600;
  }

  .description {
    color: var(--text-secondary);
    line-height: 1.6;
    margin-bottom: 24px;
    font-size: 0.95rem;
  }

  .form-group {
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .form-group.row {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-glass);
    padding: 16px;
    border-radius: var(--border-radius-md);
    border: 1px solid var(--border-glass);
  }

  label {
    color: var(--text-primary);
    font-weight: 500;
    font-size: 0.9rem;
  }

  textarea, select, input[type="range"] {
    background: var(--bg-color);
    color: var(--text-primary);
    border: 1px solid var(--border-color);
    border-radius: var(--border-radius-md);
    padding: 12px;
    font-family: inherit;
    transition: var(--transition-fast);
  }

  textarea:focus, select:focus {
    outline: none;
    border-color: var(--accent-color);
    box-shadow: 0 0 0 2px hsla(var(--accent-color), 0.2);
  }

  /* Toggle Switch styling */
  .switch {
    position: relative;
    display: inline-block;
    width: 50px;
    height: 28px;
  }
  .switch input { 
    opacity: 0;
    width: 0;
    height: 0;
  }
  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--bg-color);
    border: 1px solid var(--border-color);
    transition: .4s;
  }
  .slider:before {
    position: absolute;
    content: "";
    height: 20px;
    width: 20px;
    left: 3px;
    bottom: 3px;
    background-color: var(--text-muted);
    transition: .4s;
  }
  input:checked + .slider {
    background-color: var(--accent-color);
    border-color: var(--accent-color);
  }
  input:checked + .slider:before {
    transform: translateX(22px);
    background-color: #fff;
  }
  .slider.round {
    border-radius: 34px;
  }
  .slider.round:before {
    border-radius: 50%;
  }

  .actions {
    display: flex;
    gap: 12px;
    margin-top: 30px;
  }

  .btn {
    flex: 1;
    padding: 14px;
    border: none;
    border-radius: var(--border-radius-md);
    font-weight: 600;
    cursor: pointer;
    transition: var(--transition-fast);
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 8px;
    font-size: 1rem;
  }

  .btn.primary {
    background: var(--accent-color);
    color: #fff;
    box-shadow: var(--shadow-sm);
  }

  .btn.primary:hover {
    background: var(--accent-hover);
    transform: translateY(-2px);
    box-shadow: var(--shadow-md);
  }

  .btn.secondary.outline {
    background: transparent;
    border: 1px solid var(--accent-color);
    color: var(--accent-color);
  }

  .btn.secondary.outline:hover {
    background: var(--bg-glass);
    color: var(--text-primary);
    border-color: var(--text-primary);
  }

  .loading {
    text-align: center;
    padding: 40px;
    color: var(--text-muted);
    font-size: 1.1rem;
  }
</style>
