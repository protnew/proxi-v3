<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();
  import * as stores from '../stores/messenger'
  import { getPubkey, getName } from '../lib/api'
  import { onPresence } from '../lib/api'
  let onlineUsers = $state<Set<string>>(new Set())
  onPresence((pk: string, online: boolean) => {
    const next = new Set(onlineUsers)
    if (online) next.add(pk)
    else next.delete(pk)
    onlineUsers = next
  })
  import { searchMessages, type SearchResult } from '../lib/search'
  import VpnPanel from './VpnPanel.svelte'

  let myId = $state(getPubkey() || '')
  let copied = $state(false)
  $effect(() => {
    const id = getPubkey() || ''
    if (id) myId = id
    const t = setInterval(() => {
      const n = getPubkey() || ''
      if (n && n !== myId) myId = n
    }, 1000)
    return () => clearInterval(t)
  })
  function copyMyId() {
    if (!myId) return
    navigator.clipboard.writeText(myId)
    copied = true
    setTimeout(() => { copied = false }, 1500)
  }

  let search = $state('')
  let sortedList = $state<stores.ChatView[]>([])
  let currentActiveId = $state<string | null>(null)
  let searchResults = $state<SearchResult[]>([])
  let showSearch = $state(false)

  // Subscribe to stores reactively
  stores.sortedChats.subscribe(v => { sortedList = v })
  stores.activeChatId.subscribe(v => { currentActiveId = v })

  let filtered = $derived(
    sortedList.filter(c =>
      c.name.toLowerCase().includes(search.toLowerCase()) ||
      (c.lastMessage?.text ?? '').toLowerCase().includes(search.toLowerCase())
    )
  )

  // Search messages when query is long enough
  $effect(() => {
    if (search.length >= 3) {
      searchResults = searchMessages(search)
      showSearch = true
    } else {
      searchResults = []
      showSearch = false
    }
  })

  function selectChat(id: string) {
    dispatch('chatselect');

    stores.activeChatId.set(id)
    stores.markRead(id)
  }

  function toggleSettings() {
    stores.showSettings.update(v => !v)
  }

  function formatTime(ts: number): string {
    if (!ts) return ''
    const d = new Date(ts)
    const now = new Date()
    if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    return d.toLocaleDateString([], { day: 'numeric', month: 'short' })
  }
</script>

<aside class="sidebar">
  <div class="header">
    <button class="menu-btn" onclick={toggleSettings}>☰</button>
    <input type="text" placeholder="Поиск" bind:value={search} />
    <button class="new-btn" onclick={() => stores.showNewChat.update(() => true)}>✏️</button>
  </div>

  <div class="my-id-bar" title={myId ? ('Полный ID: ' + myId + ' — копируй только 📋') : 'identity loading...'}>
    <span class="my-id-label">Мой ID</span>
    <code class="my-id-val">{myId ? (myId.slice(0, 12) + '…' + myId.slice(-4)) : '…'}</code>
    <button type="button" class="copy-id" onclick={copyMyId} disabled={!myId} title="Копировать ПОЛНЫЙ ID">{copied ? '✓' : '📋'}</button>
  </div>
  {#if myId}
    <div class="my-id-hint">В чат вставляй только из 📋 — на экране обрезка</div>
  {/if}

  <div class="chat-list">
    {#if showSearch && searchResults.length > 0}
      <div class="search-results">
        <div class="search-header">🔍 Результаты ({searchResults.length})</div>
        {#each searchResults as r}
          <button class="search-item" onclick={() => { selectChat(r.chatId); search = ''; showSearch = false }}>
            <div class="si-chat">{r.chatName}</div>
            <div class="si-text">...{r.message.text.slice(Math.max(0, r.matchIndex - 20), r.matchIndex + 40)}...</div>
          </button>
        {/each}
      </div>
    {:else}
      {#each filtered as chat (chat.id)}
      <button class="chat-item" class:active={currentActiveId === chat.id} onclick={() => selectChat(chat.id)}>
        <div class="avatar" style="background:{chat.type === 'group' ? '#2a4a6a' : '#3a5a3a'}">
          {chat.avatar}
        </div>
        <div class="info">
          <div class="top-row">
            <span class="name">{chat.name}</span>
            {#if chat.lastMessage}
              <span class="time">{formatTime(chat.lastMessage.timestamp)}</span>
            {/if}
          </div>
          <div class="bottom-row">
            <span class="last-msg">
              {#if chat.typing && chat.typing.length > 0}
                <em>печатает...</em>
              {:else if chat.lastMessage}
                {chat.lastMessage.text.slice(0, 40)}
              {:else}
                Нет сообщений
              {/if}
            </span>
            {#if chat.unread > 0}
              <span class="badge">{chat.unread}</span>
            {/if}
          </div>
        </div>
      </button>
    {/each}
    {/if}

    {#if filtered.length === 0}
      <div class="empty">
        {search ? 'Ничего не найдено' : 'Нет чатов. Нажми ✏️'}
      </div>
    {/if}
  </div>

  <VpnPanel />
</aside>

<style>
  .sidebar { width: 360px; min-width: 360px; background: var(--bg-secondary); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); display: flex; flex-direction: column; border-right: 1px solid var(--border); height: 100vh; z-index: 20; }
  .header { display: flex; align-items: center; gap: 8px; padding: 12px; border-bottom: 1px solid var(--border); }
  .menu-btn, .new-btn { background: none; border: none; color: var(--text-muted); font-size: 20px; cursor: pointer; padding: 6px; border-radius: 50%; transition: background 0.2s; }
  .menu-btn:hover, .new-btn:hover { background: var(--bg-hover); color: var(--text); }
  input { flex: 1; background: var(--bg-tertiary); border: 1px solid transparent; color: var(--text); padding: 10px 14px; border-radius: 20px; font-size: 13px; outline: none; transition: all 0.2s; box-shadow: inset 0 2px 4px rgba(0,0,0,0.1); }
  input:focus { border-color: var(--accent); background: var(--bg); box-shadow: 0 0 0 2px var(--bg-active); }
  .chat-list { flex: 1; overflow-y: auto; }
  .chat-item { display: flex; align-items: center; gap: 12px; padding: 12px; cursor: pointer; transition: all 0.2s ease; width: 100%; text-align: left; background: transparent; border: none; color: inherit; font-family: inherit; position: relative; overflow: hidden; }
  .chat-item:hover { background: var(--bg-hover); }
  .chat-item.active { background: var(--bg-active); border-left: 4px solid var(--accent); padding-left: 8px; }
  .avatar { width: 48px; height: 48px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 20px; flex-shrink: 0; box-shadow: 0 4px 10px rgba(0,0,0,0.15); transition: transform 0.2s; }
  .chat-item:hover .avatar { transform: scale(1.05); }
  .info { flex: 1; min-width: 0; }
  .top-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
  .name { font-size: 15px; font-weight: 600; color: var(--text); }
  .time { font-size: 11px; color: var(--text-muted); }
  .bottom-row { display: flex; justify-content: space-between; align-items: center; }
  .last-msg { font-size: 13px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .last-msg em { color: var(--accent); font-style: normal; }
  .badge { background: linear-gradient(135deg, var(--accent), var(--accent-light)); color: white; font-size: 11px; font-weight: 700; border-radius: 50%; min-width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; padding: 0 5px; box-shadow: 0 2px 6px rgba(59, 130, 246, 0.4); }
  .empty { padding: 40px 20px; text-align: center; color: var(--text-dim); font-size: 13px; }

  @media (max-width: 768px) {
    .sidebar { width: 100%; min-width: 100%; }
  }
  .search-results { padding: 4px 0; }
  .search-header { padding: 8px 12px; font-size: 12px; color: #7a8a9a; }
  .search-item { display: block; width: 100%; text-align: left; background: none; border: none; color: inherit; font-family: inherit; padding: 8px 12px; cursor: pointer; }
  .search-item:hover { background: #202b36; }
  .si-chat { font-size: 12px; font-weight: 600; color: #e0e0e0; }
  .si-text { font-size: 11px; color: #7a8a9a; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .c-name.online::before {
    content: '';
    display: inline-block;
    width: 8px;
    height: 8px;
    background: #4ade80;
    border-radius: 50%;
    margin-right: 6px;
  }

  .my-id-bar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid var(--border); background: var(--bg-tertiary); }
  .my-id-label { font-size: 11px; color: var(--text-muted); flex-shrink: 0; }
  .my-id-val { flex: 1; font-size: 11px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .copy-id { background: none; border: 1px solid var(--border); border-radius: 8px; cursor: pointer; padding: 2px 8px; color: var(--text); }
  .copy-id:hover { background: var(--bg-hover); }
  .copy-id:disabled { opacity: 0.4; cursor: default; }
</style>
