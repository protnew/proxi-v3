<script lang="ts">
  import * as stores from '../stores/messenger'
  import { createGroup, subscribeGroup, getName } from '../lib/api'

  let visible = $state(false)
  let groupName = $state('')
  let groupAbout = $state('')

  stores.showGroupCreate.subscribe(v => { visible = v })

  async function create() {
    const name = groupName.trim()
    if (!name) return
    const channelId = await createGroup(name, groupAbout)
    stores.ensureGroupChat(channelId, name)
    subscribeGroup(channelId)
    stores.activeChatId.set(`group:${channelId}`)
    stores.showGroupCreate.set(false)
    groupName = ''
    groupAbout = ''
  }
</script>

{#if visible}
  <div class="overlay" onclick={() => stores.showGroupCreate.set(false)}>
    <div class="dialog" onclick={(e) => e.stopPropagation()}>
      <div class="dialog-header">
        <h3>Создать группу</h3>
        <button onclick={() => stores.showGroupCreate.set(false)}>✕</button>
      </div>
      <div class="form">
        <label for="grp-name">Название</label>
        <input id="grp-name" type="text" placeholder="Название группы" bind:value={groupName} />
        <label for="grp-about">Описание</label>
        <textarea id="grp-about" placeholder="О чём эта группа" bind:value={groupAbout} rows="2"></textarea>
        <button class="create-btn" onclick={create} disabled={groupName.trim().length === 0}>
          👥 Создать группу
        </button>
        <p class="hint">Группа использует NIP-28 (Nostr channels). Все сообщения публичны.</p>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); z-index: 200; display: flex; align-items: center; justify-content: center; }
  .dialog { background: #1e2c3a; border-radius: 12px; width: 380px; }
  .dialog-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid #0e1621; }
  h3 { margin: 0; font-size: 16px; color: #e0e0e0; }
  .dialog-header button { background: none; border: none; color: #aaa; font-size: 18px; cursor: pointer; }
  .form { padding: 16px; }
  label { display: block; font-size: 12px; color: #7a8a9a; margin: 8px 0 4px; }
  input, textarea { width: 100%; background: #242f3d; border: none; color: #e0e0e0; padding: 10px; border-radius: 8px; font-size: 13px; margin-bottom: 8px; box-sizing: border-box; font-family: inherit; }
  .create-btn { width: 100%; padding: 12px; background: #2a5a3a; border: none; color: #4fae4e; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
  .create-btn:disabled { opacity: 0.4; }
  .hint { font-size: 11px; color: #555; margin-top: 8px; }
</style>
