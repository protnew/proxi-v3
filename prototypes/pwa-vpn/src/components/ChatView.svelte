<script lang="ts">
  import * as stores from '../stores/messenger'
  import { sendDM, sendTyping, getName, sendFileManifest, sendBinaryVoice, sendGroupMessage } from '../lib/api'
  import { playOutgoing } from '../lib/sounds'
  import { sendFile } from '../lib/peer-manager'
  import { startRecording as startVoiceRecord, stopRecording as stopVoiceRecord } from '../lib/voice'
  import EmojiPicker from './EmojiPicker.svelte'
  import type { Message, ChatView } from '../stores/messenger'

  let inputText = $state('')
  let messagesEl: HTMLDivElement | undefined = $state()
  let isRecording = $state(false)
  let showEmoji = $state(false)
  let contextMenu: { msgId: string; x: number; y: number } | null = $state(null)
  let replyTo: Message | null = $state(null)
  let editingMsg: Message | null = $state(null)
  let mobileShowChat = $state(false)

  // On mobile, when chat is selected, hide sidebar
  stores.activeChatId.subscribe(v => {
    if (v && window.innerWidth < 768) mobileShowChat = true
  })

  function goBack() {
    stores.activeChatId.set(null)
    mobileShowChat = false
  }
  let uploadName = $state('')
  let uploading = $state(false)
  let uploadProgress = $state(0)
  let recordingStartTime = $state(0)
  let voiceStream: MediaStream | null = null

  import { activeChat, activeChatId, profile as profileStore } from '../stores/messenger'
  import { chatApi } from '../lib/api'

  // Sync stores → local $state for template reactivity
  let currentChat = $derived($activeChat)
  let currentChatId = $derived($activeChatId)
  let currentProfile = $derived($profileStore)

  // M-003: Load message history when chat opens
  $effect(() => {
    if (currentChatId) {
      const peerId = currentChatId.replace('dm:', '').replace('group:', '')
      chatApi.getHistory(peerId, 50).then((resp: any) => {
        if (resp.data?.messages && Array.isArray(resp.data.messages)) {
          resp.data.messages.forEach((msg: any) => {
            const chatId = currentChatId
            stores.addMessage(chatId, {
              id: msg.id || `hist-${msg.timestamp}-${msg.from?.slice(0, 8) || 'unknown'}`,
              from: msg.from || msg.sender || '',
              to: msg.to || msg.recipient || '',
              text: msg.text || '',
              timestamp: msg.timestamp || Date.now() / 1000,
              type: 'text',
              read: true, // mark history as read
              forwardedFrom: msg.forwarded_from,
            })
          })
          console.log(`[chatview] Loaded ${resp.data.messages.length} messages from history`)
        }
      }).catch(() => {})
    }
  })

  // Auto-scroll
  $effect(() => {
    const count = currentChat?.messages?.length
    if (count && messagesEl) {
      requestAnimationFrame(() => {
        if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight
      })
    }
  })

  // Mark read on switch
  $effect(() => {
    if (currentChatId) stores.markRead(currentChatId)
  })

  let peerPubkey = $derived(currentChatId?.replace('dm:', '').replace('group:', '') || '')
  let isGroup = $derived(currentChatId?.startsWith('group:') ?? false)

  async function sendMessage() {
    let text = inputText.trim()
    if (!text || !currentChatId) return

    if (editingMsg) {
      stores.editMessage(currentChatId, editingMsg.id, text)
      editingMsg = null
      inputText = ''
      return
    }

    let msg: Message
    if (isGroup) {
      const groupId = currentChatId.replace('group:', '')
      msg = await sendGroupMessage(groupId, text)
      msg.replyTo = replyTo?.id
      stores.addMessage(currentChatId, msg)
    } else {
      msg = await sendDM(peerPubkey, text, replyTo?.id)
      stores.addMessage(currentChatId, msg)
    }

    playOutgoing()
    inputText = ''
    replyTo = null
    showEmoji = false
  }

  let lastTypingTime = 0;

  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    } else if (inputText.length > 0) {
      const now = Date.now()
      if (now - lastTypingTime > 3000) {
        sendTyping(peerPubkey)
        lastTypingTime = now
      }
    }
  }

  function formatTime(ts: number): string {
    return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  function isMine(msg: Message): boolean {
    return msg.from === currentProfile?.pubkey
  }

  function getDate(ts: number): string {
    const d = new Date(ts)
    const now = new Date()
    if (d.toDateString() === now.toDateString()) return 'Сегодня'
    const y = new Date(now); y.setDate(y.getDate() - 1)
    if (d.toDateString() === y.toDateString()) return 'Вчера'
    return d.toLocaleDateString('ru', { day: 'numeric', month: 'long' })
  }

  function getReplyText(msgId: string | undefined): string {
    if (!msgId || !currentChat) return ''
    const orig = currentChat.messages.find(m => m.id === msgId)
    return orig ? orig.text.slice(0, 60) : 'Сообщение'
  }

  function addEmoji(e: string) { inputText += e; showEmoji = false }

  function onContext(e: MouseEvent, msg: Message) {
    e.preventDefault()
    contextMenu = { msgId: msg.id, x: e.clientX, y: e.clientY }
  }
  function closeContext() { contextMenu = null }
  function doReply(msg: Message) { replyTo = msg; contextMenu = null }
  function doEdit(msg: Message) { editingMsg = msg; inputText = msg.text; contextMenu = null }
  function doDelete(msg: Message) { if (currentChatId) stores.deleteMessage(currentChatId, msg.id); contextMenu = null }
  function doForward(msg: Message) { navigator.clipboard.writeText(msg.text); contextMenu = null }
  function doReact(msg: Message, emoji: string) {
    if (currentChatId && currentProfile) stores.addReaction(currentChatId, msg.id, emoji, currentProfile.pubkey)
    contextMenu = null
  }

  async function startRecording() {
    try {
      voiceStream = await startVoiceRecord()
      recordingStartTime = Date.now()
      isRecording = true
    } catch { console.error('Mic denied') }
  }

  async function stopRecording() {
    isRecording = false
    if (!voiceStream) return
    const { blob, duration, url } = await stopVoiceRecord()
    if (blob.size === 0 || !currentChatId) return

    // Send as file via P2P for longer voice messages
    if (blob.size > 50000) {
      try {
        const file = new File([blob], `voice_${Date.now()}.webm`, { type: blob.type })
        await sendFile(peerPubkey, file, (sent, total) => {
          uploadProgress = Math.round(sent / total * 100)
        })
      } catch {
        // Fallback to API upload
        try {
          const msg = await sendBinaryVoice(peerPubkey, duration, blob)
          stores.addMessage(currentChatId, msg)
        } catch (e) { console.error('Voice send failed', e) }
      }
    } else {
      // Short voice: API upload
      try {
        const msg = await sendBinaryVoice(peerPubkey, duration, blob)
        stores.addMessage(currentChatId, msg)
      } catch (e) { console.error('Voice send failed', e) }
    }
    playOutgoing()
  }

  async function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    if (!file || !currentChatId) return

    const isImage = file.type.startsWith('image/')
    const msgType = isImage ? 'image' : 'file'

    // Create thumbnail for images
    let thumbnail = ''
    if (isImage) {
      thumbnail = URL.createObjectURL(file)
    }

    // Small files (<100KB): local blob URL + Nostr manifest
    if (file.size < 100_000) {
      const url = thumbnail || URL.createObjectURL(file)
      const msg: Message = {
        id: crypto.randomUUID(),
        from: currentProfile?.pubkey || '',
        to: currentChatId,
        text: isImage ? '🖼️ Фото' : `📎 ${file.name}`,
        timestamp: Date.now(),
        type: msgType,
        fileName: file.name,
        fileSize: file.size,
        fileUrl: url,
        read: true,
      }
      stores.addMessage(currentChatId, msg)
      sendFileManifest(peerPubkey, file.name, file.size, url)
      playOutgoing()
    } else {
      // Large files: send via P2P DataChannel
      uploading = true
      uploadName = file.name
      uploadProgress = 0
      try {
        await sendFile(peerPubkey, file, (sent, total) => {
          uploadProgress = Math.round(sent / total * 100)
        })
        const msg: Message = {
          id: crypto.randomUUID(),
          from: currentProfile?.pubkey || '',
          to: currentChatId,
          text: isImage ? '🖼️ Фото' : `📎 ${file.name}`,
          timestamp: Date.now(),
          type: msgType,
          fileName: file.name,
          fileSize: file.size,
          fileUrl: thumbnail,
          read: true,
        }
        stores.addMessage(currentChatId, msg)
        playOutgoing()
      } catch (err) {
        console.error('File send failed:', err)
        const url = thumbnail || URL.createObjectURL(file)
        const msg: Message = {
          id: crypto.randomUUID(),
          from: currentProfile?.pubkey || '',
          to: currentChatId,
          text: isImage ? '🖼️ Фото' : `📎 ${file.name}`,
          timestamp: Date.now(),
          type: msgType,
          fileName: file.name,
          fileSize: file.size,
          fileUrl: url,
          read: true,
        }
        stores.addMessage(currentChatId, msg)
      }
      uploading = false
      uploadProgress = 0
    }
    input.value = ''
  }

  function formatSize(bytes: number): string {
    if (!bytes) return ''
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / 1048576).toFixed(1) + ' MB'
  }

  function downloadFile(msg: Message) {
    if (!msg.fileUrl) return
    const a = document.createElement('a')
    a.href = msg.fileUrl
    a.download = msg.fileName || 'file'
    a.click()
  }
</script>

<svelte:window onclick={closeContext} onkeydown={(e) => e.key === 'Escape' && (contextMenu = null)} onpaste={(e) => {
  const items = e.clipboardData?.items
  if (!items) return
  for (const item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) handleFileSelect({ target: { files: [file], value: '' } } as any)
      break
    }
  }
}} />

{#if currentChat}
  <div class="chat-area">
    <div class="chat-header">
      <button class="back-btn" onclick={goBack}>←</button>
      <div class="avatar">{currentChat.avatar}</div>
      <div class="peer-info">
        <span class="peer-name">{currentChat.name}</span>
        <span class="peer-status">
          {#if currentChat.typing && currentChat.typing.length > 0}
            <em>печатает...</em>
          {:else}
            был(а) недавно
          {/if}
        </span>
      </div>
      <button class="hbtn">📞</button>
      <button class="hbtn">🔍</button>
      <button class="hbtn">⋮</button>
    </div>

    <div class="messages" bind:this={messagesEl}
      ondragover={(e) => { e.preventDefault(); e.stopPropagation() }}
      ondrop={(e) => {
        e.preventDefault(); e.stopPropagation()
        const file = e.dataTransfer?.files?.[0]
        if (file) handleFileSelect({ target: { files: [file], value: '' } } as any)
      }}
    >
      {#each currentChat.messages as msg, i (msg.id)}
        {@const ds = getDate(msg.timestamp)}
        {@const prevDs = i > 0 ? getDate(currentChat.messages[i - 1].timestamp) : ''}
        {#if ds !== prevDs}
          <div class="date-sep"><span>{ds}</span></div>
        {/if}
        <div class="msg-row" class:mine={isMine(msg)} oncontextmenu={(e) => onContext(e, msg)} role="article">
          <div class="bubble" class:mine={isMine(msg)}>
            {#if msg.replyTo}
              <div class="reply-ref">↩ {getReplyText(msg.replyTo)}</div>
            {/if}
            {#if msg.forwardedFrom}
              <div class="fwd-ref">↪ Переслано от {getName(msg.forwardedFrom)}</div>
            {/if}
            {#if msg.type === 'text'}
              <div class="msg-text">{msg.text}</div>
              {#if msg.edited}<span class="edited">(ред.)</span>{/if}
            {:else if msg.type === 'voice'}
              <div class="voice-msg">
                <button class="play-btn" onclick={() => { if (msg.fileUrl) new Audio(msg.fileUrl).play() }}>▶️</button>
                <div class="voice-bars">{#each Array(20) as _}<div class="bar"></div>{/each}</div>
                <span class="dur">{msg.voiceDuration || 0}s</span>
              </div>
            {:else if msg.type === 'file'}
              <button class="file-msg" onclick={() => downloadFile(msg)}>
                <span>📄</span>
                <div class="file-info">
                  <span class="fname">{msg.fileName || 'file'}</span>
                  <span class="fsize">{formatSize(msg.fileSize || 0)}</span>
                </div>
              </button>
            {:else if msg.type === 'image'}
              <div class="image-msg">
                {#if msg.fileUrl}
                  <img src={msg.fileUrl} alt={msg.fileName || 'image'} onclick={() => window.open(msg.fileUrl, '_blank')} />
                {:else}
                  <span>🖼️ {msg.fileName || 'image'} ({formatSize(msg.fileSize || 0)})</span>
                {/if}
              </div>
            {/if}
            {#if msg.reactions && Object.keys(msg.reactions).length > 0}
              <div class="reactions">
                {#each Object.entries(msg.reactions) as [emoji, pks]}
                  <button class="react" onclick={() => doReact(msg, emoji)}>{emoji} {pks.length}</button>
                {/each}
              </div>
            {/if}
            <div class="meta">
              <span class="mtime">{formatTime(msg.timestamp)}</span>
              {#if isMine(msg)}<span class="check">{msg.read ? '✓✓' : '✓'}</span>{/if}
            </div>
          </div>
        </div>
      {/each}

      {#if currentChat.messages.length === 0}
        <div class="no-msg">
          <div>💬</div>
          <p>Начните разговор</p>
          <p class="hint">E2E зашифровано через Nostr</p>
        </div>
      {/if}
    </div>

    {#if contextMenu}
      <div class="ctx-menu" style="left:{contextMenu.x}px;top:{contextMenu.y}px">
        <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doReply(m) }}>↩ Ответить</button>
        <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doForward(m) }}>↪ Копировать</button>
        {#if currentChat?.messages.find(x => x.id === contextMenu?.msgId && isMine(x))}
          <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doEdit(m) }}>✏️ Редактировать</button>
          <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doDelete(m) }}>🗑️ Удалить</button>
        {/if}
        <div class="react-row">
          {#each ['❤️','👍','😂','😮','😢','🔥'] as r}
            <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doReact(m, r) }}>{r}</button>
          {/each}
        </div>
      </div>
    {/if}

    {#if replyTo}
      <div class="reply-bar">
        <span>↩ {replyTo.text.slice(0, 50)}{replyTo.text.length > 50 ? '...' : ''}</span>
        <button onclick={() => replyTo = null}>✕</button>
      </div>
    {/if}

    {#if editingMsg}
      <div class="reply-bar editing">
        <span>✏️ Редактирование: {editingMsg.text.slice(0, 50)}</span>
        <button onclick={() => { editingMsg = null; inputText = '' }}>✕</button>
      </div>
    {/if}

    <div class="input-area">
      <button class="ibtn" onclick={() => showEmoji = !showEmoji}>😊</button>
      <EmojiPicker {showEmoji} onSelect={addEmoji} />
      <button class="ibtn" onclick={() => document.getElementById('f-in')?.click()}>📎</button>
      <input id="f-in" type="file" hidden onchange={handleFileSelect} />
      <textarea
        placeholder={editingMsg ? 'Редактировать сообщение...' : 'Сообщение'}
        bind:value={inputText}
        onkeydown={handleKey}
        rows="1"
      ></textarea>
      {#if inputText.trim()}
        <button class="send-btn" onclick={sendMessage}>➤</button>
      {:else}
        <button class="mic-btn" class:rec={isRecording}
          onmousedown={startRecording} onmouseup={stopRecording} onmouseleave={() => isRecording && stopRecording()}>
          {isRecording ? '⏺' : '🎤'}
        </button>
      {/if}
    </div>

    <!-- Upload progress -->
    {#if uploading}
      <div class="upload-bar">
        <span>📤 {uploadName}</span>
        <div class="progress-track"><div class="progress-fill" style="width:{uploadProgress}%"></div></div>
        <span>{uploadProgress}%</span>
      </div>
    {/if}
  </div>
{:else}
  <div class="empty">
    <div class="empty-icon">🛡️</div>
    <h2>Indestructible Messenger</h2>
    <p>Выберите чат или начните новый</p>
    <p class="sub">P2P • E2E • Неубиваемо</p>
  </div>
{/if}

<style>
  .chat-area { flex: 1; display: flex; flex-direction: column; height: 100vh; background: var(--bg); position: relative; }
  .chat-header { display: flex; align-items: center; gap: 12px; padding: 10px 20px; background: var(--bg-secondary); backdrop-filter: blur(12px); border-bottom: 1px solid var(--border); z-index: 10; box-shadow: 0 4px 20px rgba(0,0,0,0.05); }
  .avatar { width: 42px; height: 42px; border-radius: 50%; background: linear-gradient(135deg, var(--accent), var(--accent-light)); display: flex; align-items: center; justify-content: center; font-size: 18px; box-shadow: 0 2px 8px rgba(0,0,0,0.2); }
  .peer-info { flex: 1; }
  .peer-name { display: block; font-size: 15px; font-weight: 600; color: var(--text); }
  .peer-status { font-size: 12px; color: var(--text-muted); }
  .peer-status em { color: var(--accent); font-style: normal; }
  .hbtn { background: none; border: none; color: var(--text-muted); font-size: 18px; cursor: pointer; width: auto; padding: 6px; border-radius: 50%; transition: background 0.2s; }
  .hbtn:hover { background: var(--bg-hover); color: var(--text); }
  .back-btn { display: none; background: none; border: none; color: var(--text-muted); font-size: 20px; cursor: pointer; padding: 4px 8px; }

  @media (max-width: 768px) {
    .back-btn { display: block; }
  }
  .messages { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 8px; }
  .date-sep { text-align: center; margin: 16px 0; }
  .date-sep span { background: var(--bg-secondary); backdrop-filter: blur(8px); color: var(--text-muted); font-size: 12px; padding: 4px 12px; border-radius: 12px; border: 1px solid var(--border); }
  .msg-row { display: flex; animation: fadeInUp 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275); }
  @keyframes fadeInUp { from { opacity: 0; transform: translateY(15px); } to { opacity: 1; transform: translateY(0); } }
  .msg-row.mine { justify-content: flex-end; }
  .bubble { max-width: 65%; padding: 10px 14px; border-radius: 16px; background: var(--bubble); position: relative; box-shadow: 0 2px 8px rgba(0,0,0,0.08); border: 1px solid var(--border); color: var(--text); }
  .bubble.mine { background: var(--bubble-mine); border-bottom-right-radius: 4px; border: none; box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3); color: white; }
  .msg-text { font-size: 14px; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
  .edited { font-size: 10px; opacity: 0.7; }
  .meta { display: flex; align-items: center; justify-content: flex-end; gap: 6px; margin-top: 4px; }
  .mtime { font-size: 11px; opacity: 0.6; }
  .check { font-size: 13px; color: #10b981; text-shadow: 0 1px 2px rgba(0,0,0,0.2); }
  .reply-ref { font-size: 12px; opacity: 0.9; border-left: 2px solid currentColor; padding-left: 8px; margin-bottom: 6px; background: rgba(0,0,0,0.1); padding: 4px 8px; border-radius: 4px; }
  .fwd-ref { font-size: 11px; opacity: 0.8; margin-bottom: 4px; font-style: italic; }
  .reactions { display: flex; gap: 4px; margin-top: 6px; flex-wrap: wrap; }
  .react { background: var(--bg-secondary); padding: 2px 8px; border-radius: 12px; font-size: 12px; cursor: pointer; border: 1px solid var(--border); color: var(--text); transition: all 0.2s; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
  .react:hover { transform: scale(1.1); border-color: var(--accent); }
  .voice-msg { display: flex; align-items: center; gap: 8px; min-width: 180px; }
  .play-btn { background: none; border: none; font-size: 18px; cursor: pointer; width: auto; padding: 0; }
  .voice-bars { display: flex; align-items: center; gap: 2px; flex: 1; }
  .bar { width: 3px; background: #7a8a9a; border-radius: 2px; height: 12px; }
  .dur { font-size: 11px; color: #7a8a9a; }
  .file-msg { display: flex; align-items: center; gap: 8px; cursor: pointer; padding: 4px; border-radius: 6px; background: none; border: none; color: inherit; font-family: inherit; }
  .file-msg:hover { background: rgba(255,255,255,0.05); }
  .file-info { display: flex; flex-direction: column; text-align: left; }
  .fname { font-size: 13px; color: #3a9aff; }
  .fsize { font-size: 11px; color: #7a8a9a; }
  .image-msg img { max-width: 280px; max-height: 280px; border-radius: 8px; cursor: pointer; display: block; }
  .image-msg img:hover { opacity: 0.9; }
  .no-msg { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; color: #555; font-size: 48px; }
  .no-msg p { font-size: 14px; color: #555; }
  .hint { font-size: 12px !important; color: #333 !important; }
  .ctx-menu { position: fixed; background: #1e2c3a; border: 1px solid #2a3a4a; border-radius: 8px; padding: 4px; z-index: 250; min-width: 160px; }
  .ctx-menu button { display: block; width: 100%; text-align: left; background: none; border: none; color: #e0e0e0; padding: 8px 12px; border-radius: 4px; cursor: pointer; font-size: 13px; font-family: inherit; }
  .ctx-menu button:hover { background: #2a3a4a; }
  .react-row { display: flex; gap: 2px; padding: 4px; border-top: 1px solid #2a3a4a; margin-top: 4px; }
  .react-row button { display: flex; align-items: center; justify-content: center; font-size: 16px; width: auto; padding: 4px 6px; }
  .reply-bar { display: flex; align-items: center; gap: 8px; padding: 8px 16px; background: #17212b; border-top: 1px solid #0e1621; font-size: 13px; color: #3a9aff; }
  .reply-bar.editing { color: #ffaa00; }
  .reply-bar span { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .reply-bar button { background: none; border: none; color: #aaa; cursor: pointer; width: auto; }
  .input-area { position: relative; display: flex; align-items: flex-end; gap: 10px; padding: 12px 20px; background: var(--bg-secondary); backdrop-filter: blur(16px); border-top: 1px solid var(--border); z-index: 10; }
  textarea { flex: 1; background: var(--bg-tertiary); border: 1px solid var(--border); color: var(--text); padding: 12px 16px; border-radius: 24px; font-size: 14px; resize: none; max-height: 120px; font-family: inherit; outline: none; line-height: 1.4; transition: all 0.2s; box-shadow: inset 0 2px 5px rgba(0,0,0,0.05); }
  textarea:focus { background: var(--bg); border-color: var(--accent); box-shadow: 0 0 0 2px var(--bg-active); }
  .ibtn { background: none; border: none; color: var(--text-muted); font-size: 20px; cursor: pointer; width: auto; padding: 8px; border-radius: 50%; transition: background 0.2s; }
  .ibtn:hover { background: var(--bg-hover); color: var(--text); }
  .send-btn { background: linear-gradient(135deg, var(--accent), var(--accent-light)); border: none; color: white; font-size: 18px; cursor: pointer; width: 44px; height: 44px; border-radius: 50%; display: flex; align-items: center; justify-content: center; transition: all 0.2s; box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4); }
  .send-btn:hover { transform: scale(1.05); box-shadow: 0 6px 16px rgba(59, 130, 246, 0.5); }
  .mic-btn { background: none; border: none; color: var(--text-muted); font-size: 24px; cursor: pointer; width: 44px; height: 44px; border-radius: 50%; display: flex; align-items: center; justify-content: center; transition: all 0.2s; }
  .mic-btn:hover { background: var(--bg-hover); color: var(--text); }
  .mic-btn.rec { color: var(--danger); animation: pulse 1s infinite; background: rgba(239, 68, 68, 0.1); }
  @keyframes pulse { 0%, 100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(239, 68, 68, 0.4); } 50% { transform: scale(1.1); box-shadow: 0 0 0 10px rgba(239, 68, 68, 0); } }
  .empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; }
  .empty-icon { font-size: 64px; margin-bottom: 16px; }
  .empty h2 { color: #e0e0e0; font-size: 22px; margin: 0 0 8px; }
  .empty p { color: #7a8a9a; font-size: 14px; margin: 2px 0; }
  .sub { color: #555 !important; font-size: 12px !important; }
  .upload-bar { display: flex; align-items: center; gap: 8px; padding: 6px 16px; background: #17212b; border-top: 1px solid #0e1621; font-size: 12px; color: #7a8a9a; }
  .progress-track { flex: 1; height: 4px; background: #242f3d; border-radius: 2px; overflow: hidden; }
  .progress-fill { height: 100%; background: #3a7bd5; border-radius: 2px; transition: width 0.3s; }
</style>
