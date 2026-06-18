<script>
  import { onMount } from 'svelte';
  import { connState, connText, myId, onlineUsers, groups, showToast, escHtml } from '../lib/stores.js';
  import { apiFetch } from '../lib/api.js';

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
      await apiFetch('/api/groups/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), creator: npub || 'anonymous' }),
      });
      showToast('✅ Группа «' + name.trim() + '» создана!');
    } catch (e) {
      showToast('❌ ' + (e.message || 'Ошибка создания группы'));
    }
  }

  onMount(() => {
    loadIdentity();
  });
</script>

<div class="identity-panel">
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
        <span style="color:#555">Только ты онлайн</span>
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
  .conn-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    background: #111;
    border-radius: 10px;
    font-size: 12px;
    margin-bottom: 16px;
  }
  .conn-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .conn-dot.on { background: #4caf50; }
  .conn-dot.off { background: #f44336; }
  .conn-dot.trying { background: #ff9800; animation: pulse 1s infinite; }
  @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
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
  .id-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
  }
  .id-row label {
    font-size: 11px;
    color: #555;
    width: 40px;
    flex-shrink: 0;
  }
  .id-row .val {
    flex: 1;
    font-family: monospace;
    font-size: 12px;
    color: #4fc3f7;
    background: #0a0a0a;
    padding: 8px 12px;
    border-radius: 8px;
    word-break: break-all;
  }
  .copy-btn {
    padding: 6px 12px;
    background: #1a1a1a;
    border: 1px solid #333;
    color: #888;
    border-radius: 8px;
    cursor: pointer;
    font-size: 11px;
  }
  .copy-btn:hover {
    background: #222;
    color: #e0e0e0;
  }
  .seed-box {
    background: #1a0a0a;
    border: 1px solid #f44336;
    border-radius: 10px;
    padding: 16px;
    margin-top: 12px;
  }
  .seed-box p {
    font-size: 11px;
    color: #f44336;
    margin-bottom: 8px;
  }
  .seed-words {
    font-family: monospace;
    font-size: 13px;
    color: #e0e0e0;
    line-height: 1.8;
  }
  .groups-create-btn {
    width: 100%;
    padding: 10px;
    background: #1a1a1a;
    color: #4fc3f7;
    border: 1px dashed #333;
    border-radius: 10px;
    cursor: pointer;
    font-size: 13px;
    transition: all 0.2s;
    margin-bottom: 10px;
  }
  .groups-create-btn:hover {
    background: #222;
    border-color: #1e88e5;
    color: #e0e0e0;
  }
  .empty-groups {
    color: #333;
    font-size: 12px;
    padding: 16px;
    text-align: center;
  }
  .group-card {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px;
    background: #0a0a0a;
    border-radius: 8px;
    margin-bottom: 6px;
    cursor: pointer;
    border: 1px solid #1a1a1a;
    transition: all 0.2s;
  }
  .group-card:hover {
    border-color: #1e88e5;
    background: #0d1520;
  }
  .g-icon { font-size: 20px; }
  .g-info { flex: 1; }
  .g-name { font-size: 13px; font-weight: 500; }
  .g-meta { font-size: 11px; color: #555; }
  .group-members-panel {
    margin-top: 12px;
    padding: 12px;
    background: #0a0a0a;
    border: 1px solid #222;
    border-radius: 10px;
  }
  .group-members-panel h4 {
    font-size: 12px;
    color: #888;
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .gm-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 0;
    font-size: 13px;
    border-bottom: 1px solid #111;
  }
  .gm-item:last-child { border-bottom: none; }
  .gm-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #4caf50;
  }
  .gm-role {
    font-size: 10px;
    color: #1e88e5;
    margin-left: auto;
    padding: 2px 6px;
    background: #0d1520;
    border-radius: 4px;
  }
  .back-to-groups {
    padding: 6px 12px;
    background: #1a1a1a;
    border: 1px solid #333;
    color: #4fc3f7;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    margin-bottom: 10px;
  }
  .back-to-groups:hover {
    background: #222;
    color: #e0e0e0;
  }
</style>
