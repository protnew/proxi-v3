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
  }
  .id-card {
    background: #111;
    border-radius: 14px;
    padding: 20px;
    border: 1px solid #222;
    margin-bottom: 16px;
  }
  .id-card h3 {
    font-size: 13px;
    color: #888;
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .form-input {
    width: 100%;
    padding: 10px 14px;
    background: #0a0a0a;
    border: 1px solid #333;
    border-radius: 8px;
    color: #e0e0e0;
    font-size: 13px;
    outline: none;
  }
  .submit-btn {
    width: 100%;
    padding: 12px;
    background: #1e88e5;
    color: #fff;
    border: none;
    border-radius: 10px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s;
  }
  .submit-btn:hover {
    background: #1565c0;
  }
  .empty {
    color: #333;
    font-size: 12px;
    padding: 20px;
    text-align: center;
  }
  .channel-card {
    padding: 16px;
    background: #111;
    border-radius: 12px;
    border: 1px solid #1a1a1a;
    margin-bottom: 10px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .channel-card:hover {
    border-color: #1e88e5;
    background: #151515;
  }
  .channel-name {
    font-size: 14px;
    font-weight: 500;
    margin-bottom: 4px;
  }
  .channel-desc {
    font-size: 12px;
    color: #666;
  }
  .channel-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 8px;
  }
  .channel-meta {
    font-size: 11px;
    color: #444;
  }
  .sub-btn {
    padding: 4px 12px;
    background: #1e88e5;
    color: #fff;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
  }
  .sub-btn:hover {
    background: #1565c0;
  }
  .channel-unread {
    margin-top: 6px;
    font-size: 11px;
    color: #4fc3f7;
    font-weight: 500;
  }
</style>
