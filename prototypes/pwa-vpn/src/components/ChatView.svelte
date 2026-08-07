<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();
  import "./ChatView.css";
  import * as stores from '../stores/messenger'
  import { sendDM, sendTyping, getName, sendFileManifest, sendBinaryVoice, sendGroupMessage, getStatus } from '../lib/api'
  import { playOutgoing } from '../lib/sounds'
  import { formatTime, formatDay, getDate, formatSize, extractUrls } from '../lib/chat-utils'
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

  $effect(() => {
    if (currentChatId) {
      const peerId = currentChatId.replace('dm:', '').replace('group:', '')
      chatApi.getHistory(peerId, 50).then((resp: any) => {
        if (resp.data?.messages && Array.isArray(resp.data.messages)) {
          resp.data.messages.forEach((msg: any) => {
            const text = (msg.text || '').trim()
            if (!text) return // MSG-006: never render empty bubbles
            const chatId = currentChatId
            stores.addMessage(chatId, {
              id: msg.id || `hist-${msg.timestamp}-${msg.from?.slice(0, 8) || 'unknown'}`,
              from: msg.from || msg.sender || '',
              to: msg.to || msg.recipient || '',
              text,
              timestamp: msg.timestamp || Date.now() / 1000,
              type: 'text',
              read: true,
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

    // Optimistic UI first — never leave text stuck in input
    const myPk = currentProfile?.pubkey || ''
    const ts = Date.now()
    const localId = `local-${ts}-${Math.random().toString(36).slice(2, 8)}`
    const peer = isGroup ? currentChatId.replace('group:', '') : peerPubkey
    const msg: Message = {
      id: localId,
      from: myPk,
      to: peer,
      text,
      timestamp: ts,
      type: 'text',
      read: true,
      replyTo: replyTo?.id,
    }
    stores.addMessage(currentChatId, msg)
    inputText = ''
    replyTo = null
    showEmoji = false
    playOutgoing()

    try {
      if (isGroup) {
        const resp: any = await sendGroupMessage(peer, text)
        if (resp?.status >= 400) console.error('[chatview] sendGroup failed', resp)
      } else {
        // Dual: Go first if connected; Nostr optional backup
        const go = getStatus()
        const nostr = (window as any).__nostrChat
        let sent = false
        if (go?.connected) {
          const resp: any = await sendDM(peer, text)
          if (resp?.status >= 400) console.error('[chatview] sendDM failed', resp)
          else sent = true
        }
        if (nostr?.isConnected) {
          try { if (await nostr.sendDM(peer, text)) sent = true } catch {}
        }
        if (!sent) {
          const resp: any = await sendDM(peer, text)
          if (resp?.status >= 400) console.error('[chatview] sendDM fallback failed', resp)
        }
      }
    } catch (e) {
      console.error('[chatview] sendMessage error', e)
    }
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

  function isMine(msg: Message): boolean {
    return msg.from === currentProfile?.pubkey
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

  function downloadFile(msg: Message) {
    if (!msg.fileUrl) return
    const a = document.createElement('a')
    a.href = msg.fileUrl
    a.download = msg.fileName || 'file'
    a.click()
  }

// M-011: URL preview — extract URLs from message text

let urlPreviews = $state<Record<string, any>>({});

async function fetchUrlPreview(url: string) {
  if (urlPreviews[url]) return;
  try {
    const resp = await fetch(API_BASE + '/api/url-preview?url=' + encodeURIComponent(url));
    if (resp.ok) {
      const data = await resp.json();
      urlPreviews[url] = data;
    }
  } catch (e) { /* ignore */ }
}

function processMessageUrls(text: string) {
  const urls = extractUrls(text);
  urls.forEach(u => fetchUrlPreview(u));
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
      <button class="back-btn" onclick={() => dispatch('back')} title="Назад">‹ Назад</button>
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

