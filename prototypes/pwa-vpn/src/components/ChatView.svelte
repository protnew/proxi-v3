<script lang="ts">
  import { toast } from '../lib/toast'; // P19
  import { startCall } from '../lib/calls'; // X3 2026-09-23
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();
  import "./ChatView.css";
  import * as stores from '../stores/messenger'
  import { updateMessageDelivery } from '../stores/messenger'
  import { sendDM, sendTyping, getName, sendFileManifest, sendBinaryVoice, sendGroupMessage, getSeckey } from '../lib/api'
  import { uploadFile, downloadByCID, formatCIDShort, MAX_FILE_BYTES } from '../lib/ipfs-storage'
  import { enqueue as outboxEnqueue, startOutboxWatcher } from '../lib/offline-outbox'
  import { playOutgoing } from '../lib/sounds'
  import { formatTime, formatDay, getDate, formatSize, extractUrls } from '../lib/chat-utils'
  import { sendFile } from '../lib/peer-manager'
  import { startRecording as startVoiceRecord, stopRecording as stopVoiceRecord } from '../lib/voice'
  import EmojiPicker from './EmojiPicker.svelte'
  import ChatHeader from './ChatHeader.svelte'
  import MessageList from './MessageList.svelte'
  import ChatComposer from './ChatComposer.svelte'
  import type { Message, ChatView } from '../stores/messenger'
  let inputText = $state('')
  let messagesEl: HTMLDivElement | undefined = $state()
  let isRecording = $state(false)
  let showEmoji = $state(false)
  let contextMenu: { msgId: string; x: number; y: number } | null = $state(null)
  let replyTo: Message | null = $state(null)
  let editingMsg: Message | null = $state(null)
  let mobileShowChat = $state(false)
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
  $effect(() => {
    const count = currentChat?.messages?.length
    if (count && messagesEl) {
      requestAnimationFrame(() => {
        if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight
      })
    }
  })
  $effect(() => {
    if (currentChatId) stores.markRead(currentChatId)
  })
  let peerPubkey = $derived(currentChatId?.replace('dm:', '').replace('group:', '') || '')
  let isGroup = $derived(currentChatId?.startsWith('group:') ?? false)
  
  // P19: header button handlers (no Amnezia; calls gated — toast only)
  let showSearch = $state(false)
  let searchQ = $state('')
  let searchHits: Array<{ chatId: string; chatName: string; message: { id: string; text: string } }> = $state([])
  let showHeaderMenu = $state(false)
  function onSearchClick() {
    showSearch = !showSearch
    showHeaderMenu = false
    if (showSearch) {
      searchHits = searchMessages(searchQ || '') as any
      toast(showSearch ? 'Поиск по сообщениям' : 'Поиск закрыт', 'info')
    }
  }
  function onSearchInput() {
    searchHits = searchMessages(searchQ) as any
  }
  function onMenuClick() {
    showHeaderMenu = !showHeaderMenu
    showSearch = false
  }
  function onCallClick() {
    // X3 CONFIRMED 2026-09-23 — wire 📞 to startCall (no Amnezia)
    if (!peerPubkey) {
      toast('Нет собеседника для звонка', 'warn')
      return
    }
    void startCall(peerPubkey, true).catch(() => {
      toast('Не удалось начать звонок', 'error')
    })
  }
  async function sendMessage() {
    let text = inputText.trim()
    if (!text || !currentChatId) return
    if (editingMsg) {
      stores.editMessage(currentChatId, editingMsg.id, text)
      editingMsg = null
      inputText = ''
      return
    }
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
      read: false,
      deliveryStatus: 'pending',
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
        if (resp?.status >= 400) {
          console.error('[chatview] sendGroup failed', resp)
          try { toast('Не удалось отправить в группу', 'error'); updateMessageDelivery(currentChatId, localId, 'failed') } catch {}
        }
      } else {
        // P4: single DM factory — server /ws only (NostrChat is receive-only).
        let sent = false
        const resp: any = await sendDM(peer, text)
        if (resp?.status >= 400) {
          console.error('[chatview] sendDM failed', resp)
          try { toast('Не удалось отправить', 'error') } catch {}
        } else if (resp?.status) {
          sent = true
        }
        if (!sent) {
          outboxEnqueue(peer, text); try { updateMessageDelivery(currentChatId, localId, 'pending'); toast('В очереди — отправится позже', 'warn') } catch {} /* P20-outbox-pending */
          console.log('[chatview] queued offline (outbox)')
        }
      }
    } catch (e) {
      console.error('[chatview] sendMessage error', e); try { toast('Ошибка отправки', 'error'); updateMessageDelivery(currentChatId, localId, 'failed') } catch {}
      try { if (!isGroup) outboxEnqueue(peerPubkey, text) } catch {}
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
  function isMine(msg: Message): boolean { return msg.from === currentProfile?.pubkey }
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
    if (blob.size > 50000) {
      try {
        const file = new File([blob], `voice_${Date.now()}.webm`, { type: blob.type })
        await sendFile(peerPubkey, file, (sent, total) => {
          uploadProgress = Math.round(sent / total * 100)
        })
      } catch {
        try {
          const msg = await sendBinaryVoice(peerPubkey, duration, blob)
          stores.addMessage(currentChatId, msg)
        } catch (e) { console.error('Voice send failed', e) }
      }
    } else {
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
    if (file.size > MAX_FILE_BYTES) {
      console.error('[chatview] file too large', file.size); try { toast('Файл слишком большой', 'error') } catch {}
      alert(`Файл слишком большой (макс ${Math.round(MAX_FILE_BYTES/1024/1024)}MB)`)
      input.value = ''
      return
    }
    uploading = true
    uploadName = file.name
    uploadProgress = 0
    try {
      const result = await uploadFile(file, {
        senderPubkey: currentProfile?.pubkey || '',
        recipientPubkey: peerPubkey,
        senderPrivateKey: getSeckey() || undefined,
        onProgress: (pct) => { uploadProgress = pct },
      })
      const preview = result.previewUrl || (isImage ? URL.createObjectURL(file) : '')
      const msg: Message = {
        id: crypto.randomUUID(),
        from: currentProfile?.pubkey || '',
        to: currentChatId,
        text: isImage ? `🖼️ ${file.name}` : `📎 ${file.name} (${formatCIDShort(result.cid)})`,
        timestamp: Date.now(),
        type: msgType,
        fileName: file.name,
        fileSize: file.size,
        fileUrl: preview || undefined,
        cid: result.cid,
        read: true,
      }
      stores.addMessage(currentChatId, msg)
      try {
        await sendFileManifest(peerPubkey, {
          name: file.name,
          size: file.size,
          cid: result.cid,
          mimeType: result.mimeType,
        } as any)
      } catch {
        try { sendFileManifest(peerPubkey, file.name, file.size, result.cid as any) } catch {}
      }
      if (file.size >= 100_000) {
        try {
          await sendFile(peerPubkey, file, (sent, total) => {
            uploadProgress = Math.round(sent / total * 100)
          })
        } catch (err) {
          console.warn('[chatview] P2P file backup failed (CID still local)', err)
        }
      }
      playOutgoing()
    } catch (err) {
      console.error('[chatview] IPFS upload failed:', err); try { toast('Не удалось загрузить файл', 'error') } catch {}
      const url = URL.createObjectURL(file)
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
    input.value = ''
  }
  async function openByCID(cid: string) {
    const got = await downloadByCID(cid)
    if (got) window.open(got.url, '_blank')
    else console.warn('[chatview] CID not found locally/gateway', cid)
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
    <ChatHeader {currentChat} {showSearch} bind:searchQ {searchHits} {showHeaderMenu}
      onBack={() => dispatch('back')} {goBack} {onCallClick} {onSearchClick} {onMenuClick} {onSearchInput}
      onAbout={() => { showHeaderMenu = false; toast('Инфо чата — скоро', 'info') }} />
        <MessageList {currentChat} bind:messagesEl {isMine} {getReplyText} {getName}
      {formatTime} {formatSize} {getDate} {onContext} {downloadFile} {doReact} {handleFileSelect} />
    {#if contextMenu}
      <div class="ctx-menu" style="left:{contextMenu.x}px;top:{contextMenu.y}px">
        <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doReply(m) }}>↩ Ответить</button>
        <button onclick={() => { const m = currentChat?.messages.find(x => x.id === contextMenu?.msgId); if (m) doForward(m) }}>Переслать</button>
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
    <ChatComposer bind:inputText bind:showEmoji {isRecording} {replyTo} {editingMsg}
      {uploading} {uploadName} {uploadProgress} {addEmoji} {handleKey} {sendMessage}
      {startRecording} {stopRecording} {handleFileSelect}
      clearReply={() => replyTo = null}
      clearEdit={() => { editingMsg = null; inputText = '' }} />
  </div>
{:else}
  <div class="empty">
    <h2 data-testid="chat-empty-brand">Proxi</h2>
    <p>Выберите чат слева или начните новый</p>
    <button type="button" class="empty-cta" data-testid="chat-empty-new" onclick={() => stores.showNewChat.set(true)}>Новый чат</button>
  </div>
{/if}
