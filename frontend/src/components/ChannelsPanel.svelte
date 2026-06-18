<script>
  import { onMount } from 'svelte';
  import { channels, myId, showToast, escHtml } from '../lib/stores.js';
  import { getChannels, apiFetch } from '../lib/api.js';

  let chName = $state('');
  let chDesc = $state('');

  async function loadChannels() {
    try {
      const data = await getChannels();
      channels.set(data.channels || data || []);
    } catch (e) {
      channels.set([]);
    }
  }

  async function createChannel() {
    if (!chName.trim()) {
      showToast('❌ Введите название канала');
      return;
    }
    try {
      await apiFetch('/api/channels', {
        method: 'POST',
        body: JSON.stringify({ name: chName.trim(), description: chDesc.trim(), creator: $myId || 'anonymous' }),
      });
      const name = chName.trim();
      chName = '';
      chDesc = '';
      showToast('✅ Канал «' + name + '» создан!');
      loadChannels();
    } catch (e) {
      showToast('❌ ' + (e.message || 'Ошибка создания канала'));
    }
  }

  async function subscribeChannel(channelId) {
    try {
      await apiFetch('/api/channels/subscribe', {
        method: 'POST',
        body: JSON.stringify({ channelId }),
      });
      showToast('📡 Вы подписались на канал');
    } catch (e) {
      showToast('❌ Ошибка подписки');
    }
  }

  onMount(() => {
    loadChannels();
  });
</script>

<div class="channels-panel">
  <h1>📡 Каналы</h1>

  <div class="id-card" style="margin-bottom:20px">
    <h3>➕ Создать канал</h3>
    <div style="margin-bottom:10px">
      <input bind:value={chName} placeholder="Название канала" class="form-input" />
    </div>
    <div style="margin-bottom:12px">
      <input bind:value={chDesc} placeholder="Описание (необязательно)" class="form-input" />
    </div>
    <button onclick={createChannel} class="submit-btn">Создать</button>
  </div>

  <div>
    {#if $channels.length === 0}
      <div class="empty">Пока нет каналов — создай первый!</div>
    {:else}
      {#each $channels as ch}
        <div class="channel-card">
          <div class="channel-name">{ch.name}</div>
          {#if ch.description}
            <div class="channel-desc">{ch.description}</div>
          {/if}
          <div class="channel-footer">
            <div class="channel-meta">
              {ch.subscribers || 0} подписчик(ов) · {ch.creator?.substring(0, 12) || 'unknown'}...
            </div>
            <button onclick={() => subscribeChannel(ch.id)} class="sub-btn">Подписаться</button>
          </div>
          {#if ch.unread && ch.unread > 0}
            <div class="channel-unread">{ch.unread} новых</div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .channels-panel {
    padding: 24px;
  }
  .channels-panel h1 {
    font-size: 20px;
    margin-bottom: 24px;
    color: var(--text-primary);
  }
  .id-card {
    background: var(--bg-panel);
    border-radius: 14px;
    padding: 20px;
    border: 1px solid var(--border-glass);
    margin-bottom: 16px;
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
    box-shadow: var(--shadow-glass);
  }
  .id-card h3 {
    font-size: 13px;
    color: var(--text-secondary);
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 600;
  }
  .form-input {
    width: 100%;
    padding: 12px 16px;
    background: var(--bg-glass);
    border: 1px solid var(--border-strong);
    border-radius: 10px;
    color: var(--text-primary);
    font-size: 13px;
    outline: none;
    transition: all 0.2s;
  }
  .form-input:focus {
    border-color: var(--accent);
    background: var(--bg-body);
    box-shadow: 0 0 10px rgba(2, 136, 209, 0.2);
  }
  .submit-btn {
    width: 100%;
    padding: 14px;
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: #ffffff;
    border: none;
    border-radius: 10px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 600;
    transition: all 0.2s;
    box-shadow: 0 4px 15px rgba(2, 136, 209, 0.3);
  }
  .submit-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(2, 136, 209, 0.5);
  }
  .empty {
    color: var(--text-muted);
    font-size: 12px;
    padding: 20px;
    text-align: center;
  }
  .channel-card {
    padding: 16px;
    background: var(--bg-body);
    border-radius: 12px;
    border: 1px solid var(--border-glass);
    margin-bottom: 10px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .channel-card:hover {
    border-color: var(--accent);
    background: var(--bg-glass);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  }
  .channel-name {
    font-size: 15px;
    font-weight: 600;
    margin-bottom: 4px;
    color: var(--text-primary);
  }
  .channel-desc {
    font-size: 12px;
    color: var(--text-secondary);
  }
  .channel-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 10px;
  }
  .channel-meta {
    font-size: 11px;
    color: var(--text-muted);
  }
  .sub-btn {
    padding: 6px 14px;
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: #ffffff;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 11px;
    font-weight: 600;
    transition: all 0.2s;
  }
  .sub-btn:hover {
    box-shadow: 0 4px 12px rgba(2, 136, 209, 0.4);
    transform: translateY(-1px);
  }
  .channel-unread {
    margin-top: 6px;
    font-size: 11px;
    color: #4fc3f7;
    font-weight: 500;
  }
</style>
