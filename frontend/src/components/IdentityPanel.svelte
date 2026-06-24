<script>
  import { onMount } from 'svelte';
  import { connState, connText, myId, onlineUsers, groups, showToast, escHtml } from '../lib/stores.js';

  import { apiFetch } from '../lib/api.js';
  import { CreateGroupSchema } from '../lib/schemas.js';

  let npub = $state('загрузка...');
  let seedCard = $state(false);
  let seedWords = $state('');
  let showGroupMembers = $state(false);
  let groupMembersTitle = $state('');
  let groupMembers = $state([]);

  $effect(() => {
    myId.subscribe((v) => (npub = v || 'загрузка...'));
  });

  function copyId(text) {
    navigator.clipboard.writeText(text).then(() => showToast('📋 Скопировано!')).catch(() => showToast('❌ Не удалось скопировать'));
  }

  async function loadIdentity() {
    try {
      const d = await apiFetch('/api/identity');
      npub = d.npub;
      if (d.isNew && d.mnemonic) {
        seedCard = true;
        seedWords = d.mnemonic;
      }
    } catch (e) {}
  }

  async function viewGroupMembers(groupId, groupName) {
    groupMembersTitle = 'Участники — ' + (groupName || 'Группа');
    groupMembers = [];
    showGroupMembers = true;

    try {
      const d = await apiFetch('/api/groups/members?group_id=' + encodeURIComponent(groupId));
      groupMembers = d.members || d || [];
    } catch (e) {
      groupMembers = [];
    }
  }

  async function createGroup() {
    const name = prompt('Название группы:');
    if (!name || !name.trim()) return;
    try {
      // Валидация Zod
      CreateGroupSchema.parse({ name: name.trim(), creatorNpub: npub || 'anonymous' });

      await apiFetch('/api/groups/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), creatorNpub: npub || 'anonymous' }),
      });
      showToast('✅ Группа «' + name.trim() + '» создана!');
    } catch (e) {
      if (e.errors) {
        showToast('❌ Ошибка валидации: ' + e.errors[0].message);
      } else {
        showToast('❌ ' + (e.message || 'Ошибка создания группы'));
      }
    }
  }

  onMount(() => {
    loadIdentity();
  });
</script>

<div class="identity-panel">
  <div class="header-actions">
    <h3>Профиль и Настройки</h3>
  </div>

  <div class="conn-status">
    <div class="conn-dot" class:on={$connState === 'on'} class:off={$connState === 'off'} class:trying={$connState === 'trying'}></div>
    <span>{$connText}</span>
  </div>

  <div class="id-card">
    <h3>🔑 Твой ID</h3>
    <div class="id-row">
      <label>npub</label>
      <div class="val">{npub}</div>
      <button class="copy-btn" onclick={() => copyId(npub)}>📋</button>
    </div>
  </div>

  <div class="id-card">
    <h3>👥 Онлайн</h3>
    <div class="online-list">
      {#if $onlineUsers.length === 0}
        <span class="text-muted">Только ты онлайн</span>
      {:else}
        {#each $onlineUsers as user}
          <div style="padding:4px 0">🟢 {user.substring(0, 16)}...</div>
        {/each}
      {/if}
    </div>
  </div>

  <div class="id-card groups-section">
    <h3>👥 Группы</h3>
    <button class="groups-create-btn" onclick={createGroup}>➕ Создать группу</button>

    {#if !showGroupMembers}
      <div id="groups-list">
        {#if $groups.length === 0}
          <div class="empty-groups">Пока нет групп — создай первую!</div>
        {:else}
          {#each $groups as g}
            <div class="group-card" onclick={() => viewGroupMembers(g.id, g.name || 'Группа')}>
              <span class="g-icon">👥</span>
              <div class="g-info">
                <div class="g-name">{g.name || 'Без имени'}</div>
                <div class="g-meta">{g.members || g.memberCount || 0} участник(ов)</div>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    {:else}
      <div class="group-members-panel show">
        <button class="back-to-groups" onclick={() => (showGroupMembers = false)}>← Назад к группам</button>
        <h4>{groupMembersTitle}</h4>
        {#each groupMembers as m}
          <div class="gm-item">
            <span class="gm-dot"></span>
            <span>{m.name || (m.id || m.userId || m.npub || '').substring(0, 16)}...</span>
            {#if m.role === 'creator' || m.role === 'admin'}
              <span class="gm-role">{m.role}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  {#if seedCard}
    <div class="id-card">
      <h3>⚠️ Seed Phrase — запиши!</h3>
      <div class="seed-box">
        <p>Это единственный способ восстановить аккаунт. Запиши на бумаге!</p>
        <div class="seed-words">{seedWords}</div>
      </div>
      <button class="copy-btn" style="margin-top:12px" onclick={() => copyId(seedWords)}>📋 Копировать seed</button>
    </div>
  {/if}
</div>

<style>
  .identity-panel {
    padding: 24px;
    max-width: 500px;
    overflow-y: auto;
  }
  .header-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }
  .header-actions h3 {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .theme-toggle {
    background: var(--bg-glass);
    border: 1px solid var(--border-glass);
    border-radius: 50%;
    width: 40px;
    height: 40px;
    font-size: 18px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
  }
  .theme-toggle:hover {
    background: var(--border-strong);
    transform: rotate(15deg);
  }
  .conn-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    background: var(--bg-glass);
    border: 1px solid var(--border-glass);
    border-radius: 12px;
    font-size: 12px;
    margin-bottom: 24px;
  }
  .conn-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .conn-dot.on { background: var(--success); }
  .conn-dot.off { background: var(--error); }
  .conn-dot.trying { background: #ffb74d; animation: pulse 1s infinite; }
  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
  .id-card {
    background: var(--bg-glass);
    border-radius: 16px;
    padding: 20px;
    border: 1px solid var(--border-glass);
    margin-bottom: 16px;
    box-shadow: 0 4px 20px var(--shadow-glass);
  }
  .id-card h3 {
    font-size: 13px;
    color: var(--text-secondary);
    margin-bottom: 16px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 600;
  }
  .id-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
  }
  .id-row label {
    font-size: 11px;
    color: var(--text-muted);
    width: 40px;
    flex-shrink: 0;
  }
  .id-row .val {
    flex: 1;
    font-family: monospace;
    font-size: 12px;
    color: var(--accent);
    background: var(--bg-body);
    padding: 10px 14px;
    border-radius: 8px;
    border: 1px solid var(--border-glass);
    word-break: break-all;
  }
  .copy-btn {
    padding: 8px 14px;
    background: var(--bg-glass);
    border: 1px solid var(--border-glass);
    color: var(--text-secondary);
    border-radius: 8px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: all 0.2s;
  }
  .copy-btn:hover {
    background: var(--border-strong);
    color: var(--text-primary);
  }
  .seed-box {
    background: rgba(211, 47, 47, 0.05);
    border: 1px solid var(--error);
    border-radius: 12px;
    padding: 16px;
    margin-top: 12px;
  }
  .seed-box p {
    font-size: 11px;
    color: var(--error);
    margin-bottom: 8px;
    font-weight: 600;
  }
  .seed-words {
    font-family: monospace;
    font-size: 13px;
    color: var(--text-primary);
    line-height: 1.8;
  }
  .groups-create-btn {
    width: 100%;
    padding: 12px;
    background: var(--bg-glass);
    color: var(--accent);
    border: 1px dashed var(--border-strong);
    border-radius: 10px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
    transition: all 0.2s;
    margin-bottom: 12px;
  }
  .groups-create-btn:hover {
    background: var(--border-strong);
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .empty-groups {
    color: var(--text-muted);
    font-size: 12px;
    padding: 16px;
    text-align: center;
  }
  .group-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px;
    background: var(--bg-body);
    border-radius: 10px;
    margin-bottom: 8px;
    cursor: pointer;
    border: 1px solid var(--border-glass);
    transition: all 0.2s;
  }
  .group-card:hover {
    border-color: var(--accent);
    background: var(--bg-glass);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  }
  .g-icon { font-size: 20px; }
  .g-info { flex: 1; }
  .g-name { font-size: 14px; font-weight: 600; color: var(--text-primary); }
  .g-meta { font-size: 11px; color: var(--text-muted); }
  .group-members-panel {
    margin-top: 16px;
    padding: 16px;
    background: var(--bg-body);
    border: 1px solid var(--border-glass);
    border-radius: 12px;
  }
  .group-members-panel h4 {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 600;
  }
  .gm-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 0;
    font-size: 13px;
    border-bottom: 1px solid var(--border-glass);
    color: var(--text-primary);
  }
  .gm-item:last-child { border-bottom: none; }
  .gm-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--success);
  }
  .gm-role {
    font-size: 10px;
    color: #ffffff;
    margin-left: auto;
    padding: 2px 8px;
    background: var(--accent);
    border-radius: 10px;
    font-weight: 600;
  }
  .back-to-groups {
    padding: 8px 14px;
    background: var(--bg-glass);
    border: 1px solid var(--border-glass);
    color: var(--accent);
    border-radius: 8px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 600;
    margin-bottom: 16px;
    transition: all 0.2s;
  }
  .back-to-groups:hover {
    background: var(--border-strong);
    color: var(--text-primary);
  }
  .text-muted {
    color: var(--text-muted);
  }
</style>
