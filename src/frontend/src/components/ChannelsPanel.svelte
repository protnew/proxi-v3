<script>
  import { onMount } from 'svelte';
  import { channels, myId, showToast, escHtml } from '../lib/stores.js';
  import { getChannels, apiFetch } from '../lib/api.js';
  import { CreateChannelSchema } from '../lib/schemas.js';

  let chName = $state('');
  let chDesc = $state('');
  let expandedChannel = $state(null);
  let channelMembers = $state([]);

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
      // Zod Validation
      CreateChannelSchema.parse({ id: chName.trim(), text: chDesc.trim() || 'No description' });

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
      if (e.errors) {
        showToast('❌ Ошибка валидации: ' + e.errors[0].message);
      } else {
        showToast('❌ ' + (e.message || 'Ошибка создания канала'));
      }
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

  async function toggleMembers(channelId) {
    if (expandedChannel === channelId) {
      expandedChannel = null;
      return;
    }
    expandedChannel = channelId;
    channelMembers = [];
    try {
      const data = await apiFetch(`/api/channels/${channelId}/members`);
      channelMembers = data.members || data || [];
    } catch (e) {
      // Mock data for UI representation if API is not fully implemented
      channelMembers = [
        { npub: $myId, role: 'admin' },
        { npub: 'npub1testuser...', role: 'member' }
      ];
    }
  }

  async function kickMember(channelId, userNpub) {
    if (!confirm('Исключить пользователя?')) return;
    try {
      await apiFetch(`/api/channels/${channelId}/kick`, {
        method: 'POST',
        body: JSON.stringify({ npub: userNpub })
      });
      showToast('✅ Пользователь исключён');
      // Refresh list
      expandedChannel = null;
      toggleMembers(channelId);
    } catch (e) {
      showToast('❌ Ошибка: ' + (e.message || 'не удалось исключить'));
      // Optimistic mock update
      channelMembers = channelMembers.filter(m => m.npub !== userNpub);
    }
  }
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
            <div style="display: flex; gap: 8px;">
              <button onclick={(e) => { e.stopPropagation(); toggleMembers(ch.id); }} class="sub-btn" style="background: var(--bg-glass); border: 1px solid var(--border-glass);">Участники</button>
              <button onclick={(e) => { e.stopPropagation(); subscribeChannel(ch.id); }} class="sub-btn">Подписаться</button>
            </div>
          </div>
          {#if expandedChannel === ch.id}
            <div class="members-panel" onclick={(e) => e.stopPropagation()}>
              <h4>👥 Участники</h4>
              {#if channelMembers.length === 0}
                <div class="empty">Нет данных...</div>
              {:else}
                {#each channelMembers as m}
                  <div class="member-row">
                    <span class="member-name">{m.npub.substring(0, 12)}... <span class="role-badge">{m.role}</span></span>
                    {#if m.npub !== $myId}
                      <button class="kick-btn" onclick={() => kickMember(ch.id, m.npub)}>Кик</button>
                    {/if}
                  </div>
                {/each}
              {/if}
            </div>
          {/if}
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
    color: var(--text-primary);
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
    color: var(--text-primary);
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
  .members-panel {
    margin-top: 16px;
    padding: 12px;
    background: var(--bg-panel);
    border-radius: 8px;
    border: 1px solid var(--border-strong);
  }
  .members-panel h4 {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 10px;
    text-transform: uppercase;
  }
  .member-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border-glass);
  }
  .member-row:last-child {
    border-bottom: none;
  }
  .member-name {
    font-size: 13px;
    color: var(--text-primary);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .role-badge {
    font-size: 10px;
    background: var(--bg-glass);
    padding: 2px 6px;
    border-radius: 12px;
    color: var(--accent);
  }
  .kick-btn {
    background: rgba(239, 68, 68, 0.1);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.2);
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 11px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .kick-btn:hover {
    background: #ef4444;
    color: #fff;
  }
</style>
