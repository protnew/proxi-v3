<script lang="ts">
  import * as stores from '../stores/messenger'
  import { getName, getUserId } from '../lib/api'

  let newKey = $state('')
  let groupName = $state('')
  let selectedMembers = $state([])
  let newName = $state('')
  let tab = $state<'new' | 'list'>('new')
  let visible = $state(false)
  let contactList = $state<stores.Contact[]>([])
  let currentProfile = $state<stores.Profile>({ pubkey: '', name: '', about: '', avatar: '👤' })

  stores.showNewChat.subscribe(v => { visible = v })
  stores.contacts.subscribe(v => { contactList = v })
  stores.profile.subscribe(v => { currentProfile = v })

  async function createGrp() {
    if (groupName.trim().length < 2) return;
    const r = await createGroupUI(groupName, selectedMembers);
    if (r.status < 300) {
      stores.showNewChat.set(false);
      groupName = '';
      selectedMembers = [];
    }
  }

  function startChat() {
    const pk = newKey.trim()
    if (!pk || pk.length < 8) return
    const name = newName.trim() || getName(pk)
    const chatId = stores.ensureDMChat(pk, name)
    stores.addContact(pk, name)
    stores.activeChatId.set(chatId)
    stores.showNewChat.set(false)
    newKey = ''; newName = ''
  }

  function openExisting(pk: string, name: string) {
    const chatId = stores.ensureDMChat(pk, name)
    stores.activeChatId.set(chatId)
    stores.showNewChat.set(false)
  }
</script>

{#if visible}
  <div class="overlay" onclick={() => stores.showNewChat.set(false)}>
    <div class="dialog" onclick={(e) => e.stopPropagation()}>
      <div class="dialog-header">
        <h3>Новый чат</h3>
        <button onclick={() => stores.showNewChat.set(false)}>✕</button>
      </div>

      <div class="tabs">
        <button class:active={tab === 'new'} onclick={() => tab = 'new'}>Новый контакт</button>
        <button class:active={tab === 'list'} onclick={() => tab = 'list'}>Контакты ({contactList.length})</button>
        <button class:active={tab === 'group'} onclick={() => tab = 'group'}>👥 Группа</button>
      </div>

      {#if tab === 'new'}
        <div class="form">
          <label for="pk-input">User ID</label>
          <textarea id="pk-input" placeholder="Вставь ID друга (из его профиля)" bind:value={newKey} rows="2"></textarea>
          <label for="name-input">Имя (необязательно)</label>
          <input id="name-input" type="text" placeholder="Имя друга" bind:value={newName} />
          <button class="start-btn" onclick={startChat} disabled={newKey.trim().length < 8}>
            💬 Начать чат
          </button>

          <div class="share-section">
            <p>Твой ключ — отправь другу:</p>
            <div class="key-box">
              <code>{getUserId() || currentProfile?.pubkey?.slice(0, 32)}...</code>
              <button onclick={() => navigator.clipboard.writeText(currentProfile?.pubkey || '')}>📋</button>
            </div>
          </div>
        </div>
            {:else if tab === 'group'}
        <div class="form">
          <label for="grp-name">Название группы</label>
          <input id="grp-name" type="text" placeholder="Моя группа" bind:value={groupName} />
          <label>Участники (выбери из контактов)</label>
          <div class="contact-list" style="max-height:200px;overflow-y:auto">
            {#each contactList as c}
              <label class="contact-check" style="display:flex;align-items:center;gap:8px;padding:6px;cursor:pointer">
                <input type="checkbox" value={c.pubkey} bind:group={selectedMembers} />
                <span>{c.name || c.pubkey.slice(0, 16)}</span>
              </label>
            {/each}
          </div>
          <button class="start-btn" onclick={createGrp} disabled={groupName.trim().length < 2}>
            👥 Создать группу
          </button>
        </div>
{:else}
        <div class="contact-list">
          {#each contactList as c}
            <button class="contact" onclick={() => openExisting(c.pubkey, c.name)}>
              <span class="c-av">{c.avatar}</span>
              <span class="c-name">{c.name}</span>
              <span class="c-pk">{c.pubkey.slice(0, 12)}...</span>
            </button>
          {/each}
          {#if contactList.length === 0}
            <p class="empty">Нет сохранённых контактов</p>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); z-index: 200; display: flex; align-items: center; justify-content: center; }
  .dialog { background: #1e2c3a; border-radius: 12px; width: 420px; max-height: 80vh; overflow-y: auto; }
  .dialog-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid #0e1621; }
  h3 { margin: 0; font-size: 16px; }
  .dialog-header button { background: none; border: none; color: #aaa; font-size: 18px; cursor: pointer; width: auto; }
  .tabs { display: flex; border-bottom: 1px solid #0e1621; }
  .tabs button { flex: 1; background: none; border: none; color: #7a8a9a; padding: 10px; cursor: pointer; font-size: 13px; }
  .tabs button.active { color: #3a9aff; border-bottom: 2px solid #3a9aff; }
  .form { padding: 16px; }
  label { display: block; font-size: 12px; color: #7a8a9a; margin: 8px 0 4px; }
  textarea, input { width: 100%; background: #242f3d; border: none; color: #e0e0e0; padding: 10px; border-radius: 8px; font-size: 13px; margin-bottom: 8px; font-family: monospace; box-sizing: border-box; }
  .start-btn { width: 100%; padding: 12px; background: #3a7bd5; border: none; color: white; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
  .start-btn:disabled { opacity: 0.4; }
  .share-section { margin-top: 16px; padding-top: 12px; border-top: 1px solid #2a3a4a; }
  .share-section p { font-size: 12px; color: #7a8a9a; margin-bottom: 6px; }
  .key-box { display: flex; gap: 6px; align-items: center; }
  .key-box code { flex: 1; background: #242f3d; padding: 8px; border-radius: 6px; font-size: 11px; word-break: break-all; }
  .key-box button { background: #242f3d; border: none; color: #aaa; padding: 6px 10px; border-radius: 6px; cursor: pointer; width: auto; }
  .contact-list { padding: 8px; }
  .contact { display: flex; align-items: center; gap: 10px; padding: 10px; border-radius: 8px; cursor: pointer; width: 100%; text-align: left; background: none; border: none; color: inherit; font-family: inherit; }
  .contact:hover { background: #242f3d; }
  .c-av { font-size: 20px; width: 36px; height: 36px; background: #3a5a3a; border-radius: 50%; display: flex; align-items: center; justify-content: center; }
  .c-name { flex: 1; font-size: 14px; }
  .c-pk { font-size: 10px; color: #555; }
  .empty { text-align: center; color: #555; padding: 30px; font-size: 13px; }
</style>
