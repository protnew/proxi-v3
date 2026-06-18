<script>
  import { onMount } from 'svelte';
  import { messages, myId, showToast, escHtml } from '../lib/stores.js';
  import { sendMessage, getWS } from '../lib/ws.js';
  import { createWebSocket, getMessages as apiGetMessages, apiFetch } from '../lib/api.js';
  import * as VoiceMessages from '../lib/voice.js';
  import * as OfflineStorage from '../lib/offline.js';
  import * as WebRTCCall from '../lib/webrtc.js';
  import Message from './Message.svelte';
  import VoiceRecorder from './VoiceRecorder.svelte';
  import WebRTC from './WebRTC.svelte';

  let inputText = $state('');
  let replyToId = $state(null);
  let replyToFrom = $state('');
  let replyToText = $state('');
  let currentTTL = $state(0);
  let showCallPanel = $state(false); // Managed internally, not bindable

  // Real-time WS via API
  let realtimeWS = $state(null);
  let realtimeStatus = $state('disconnected');

  const ttlOptions = [0, 60, 300, 3600];
  const ttlLabels = ['⏱0', '⏱1м', '⏱5м', '⏱1ч'];

  let msgListEl;
  let typingIndicator = $state('');
  let typingTimeout;

  // Connect to real-time WS via createWebSocket
  function connectRealtimeWS() {
    if (realtimeWS && realtimeWS.readyState === WebSocket.OPEN) return;

    realtimeWS = createWebSocket('/ws');
    realtimeStatus = 'connecting';

    realtimeWS.onopen = () => {
      realtimeStatus = 'connected';
    };

    realtimeWS.onmessage = async (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'chat') {
          const id = $myId;
          const isMe = msg.from === id;
          const time = new Date().toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' });
          messages.update((msgs) => [
            ...msgs,
            {
              id: msg.id || 'rt-' + Date.now(),
              from: msg.from,
              text: msg.text,
              isMe,
              isSystem: false,
              isVoice: false,
              time,
              replyTo: msg.replyTo || null,
              replyToFrom: msg.replyToFrom || null,
              replyToText: msg.replyToText || null,
              forwardedFrom: msg.forwardedFrom || null,
              ttl: msg.ttl || 0,
            },
          ]);
        } else if (msg.type === 'typing') {
          typingIndicator = msg.from?.substring(0, 12) + ' печатает...';
          clearTimeout(typingTimeout);
          typingTimeout = setTimeout(() => { typingIndicator = ''; }, 3000);
        }
      } catch {}
    };

    realtimeWS.onclose = () => {
      realtimeStatus = 'disconnected';
      // Reconnect after delay
      setTimeout(connectRealtimeWS, 3000);
    };

    realtimeWS.onerror = () => {
      realtimeStatus = 'error';
    };
  }

  // Send message via real-time WS
  function sendViaWS(text) {
    if (realtimeWS && realtimeWS.readyState === WebSocket.OPEN) {
      const msg = {
        type: 'chat',
        from: $myId,
        to: 'broadcast',
        text: text,
        ts: (Date.now() / 1000) | 0,
      };
      realtimeWS.send(JSON.stringify(msg));
    }
  }

  // Auto-scroll
  $effect(() => {
    if (msgListEl) {
      const msgs = $messages;
      requestAnimationFrame(() => {
        msgListEl.scrollTop = msgListEl.scrollHeight;
      });
    }
  });

  function send() {
    const text = inputText.trim();
    if (!text) return;
    sendMessage(text, replyToId, replyToFrom, replyToText, currentTTL);
    inputText = '';
    cancelReply();
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }

  function startReply(msgId, from, text) {
    replyToId = msgId;
    replyToFrom = from;
    replyToText = text;
  }

  function cancelReply() {
    replyToId = null;
    replyToFrom = '';
    replyToText = '';
  }

  function cycleTTL() {
    const idx = (ttlOptions.indexOf(currentTTL) + 1) % ttlOptions.length;
    currentTTL = ttlOptions[idx];
  }

  function forwardMsg(from, text) {
    const ws = getWS();
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      showToast('❌ Нет подключения');
      return;
    }
    const id = $myId;
    const msg = { type: 'chat', from: id, to: 'broadcast', text: text, ts: (Date.now() / 1000) | 0, forwardedFrom: from };
    ws.send(JSON.stringify(msg));
    showToast('↗ Сообщение переслано');
  }

  async function ipfsAttach() {
    try {
      const d = await apiFetch('/api/ipfs/status');
      if (!d.available) {
        showToast('IPFS недоступен');
        return;
      }
    } catch (e) {
      showToast('IPFS недоступен');
      return;
    }
    document.getElementById('ipfs-file-input')?.click();
  }

  async function ipfsFileSelected(input) {
    const file = input.files[0];
    if (!file) return;
    input.value = '';

    try {
      const fd = new FormData();
      fd.append('file', file);
      
      const token = localStorage.getItem('proxi_jwt') || '';
      const r = await fetch('/api/ipfs/upload', { 
        method: 'POST', 
        body: fd,
        headers: { 'Authorization': 'Bearer ' + token }
      });
      if (!r.ok) {
        const err = await r.json().catch(() => ({}));
        showToast('❌ Ошибка загрузки: ' + (err.error || r.statusText));
        return;
      }
      const d = await r.json();
      const cid = d.cid || d.hash || d.ipfsHash || '';
      if (!cid) {
        showToast('❌ Сервер не вернул CID');
        return;
      }
      const fileName = file.name || 'file';
      const sizeKB = (file.size / 1024).toFixed(1);
      const text = '📎 ' + fileName + ' (' + sizeKB + ' KB, CID: ' + cid + ')';
      sendMessage(text);
      showToast('📎 Файл загружен в IPFS!');
    } catch (e) {
      showToast('❌ Ошибка загрузки файла');
    }
  }

  function startAudioCall() {
    const peerId = prompt('ID пира для аудиозвонка:');
    if (peerId) {
      WebRTCCall.initCall(peerId, false);
      showCallPanel = true;
    }
  }

  function startVideoCall() {
    const peerId = prompt('ID пира для видеозвонка:');
    if (peerId) {
      WebRTCCall.initCall(peerId, true);
      showCallPanel = true;
    }
  }

  onMount(() => {
    WebRTCCall.setOnStateChange((state) => {
      if (state === 'idle') showCallPanel = false;
      else showCallPanel = true;
    });

    WebRTCCall.setOnIncomingCall((from) => {
      if (confirm('📞 Входящий звонок от ' + from.substring(0, 8) + '...\nПринять?')) {
        WebRTCCall.acceptCall(false);
        showCallPanel = true;
      } else {
        WebRTCCall.rejectCall();
      }
    });

    // Connect real-time WS via API
    connectRealtimeWS();

    // Load message history from API
    apiGetMessages('broadcast', 50).catch(() => {});
  });
</script>

<div class="chat-container">
  {#if showCallPanel}
    <WebRTC showPanel={showCallPanel} />
  {/if}

  <div class="messages" bind:this={msgListEl}>
    {#if $messages.length === 0}
      <div class="welcome">
        <h3>💬 Мессенджер</h3>
        <p>Nostr-протокол · P2P · Без цензуры<br />Открой с другого устройства чтобы начать</p>
      </div>
    {:else}
      {#each $messages as msg (msg.id)}
        <Message {msg} {startReply} {forwardMsg} />
      {/each}
    {/if}
  </div>

  <div class="typing">{typingIndicator}</div>

  {#if replyToId}
    <div class="reply-bar show">
      <div class="reply-preview">
        <div class="rp-from">{replyToFrom}...</div>
        <div class="rp-text">{replyToText}</div>
      </div>
      <button class="reply-cancel" onclick={cancelReply}>✕</button>
    </div>
  {/if}

  <div class="input-bar">
    <input
      bind:value={inputText}
      onkeydown={handleKeydown}
      placeholder="Написать сообщение..."
    />
    <VoiceRecorder />
    <button
      class="tool-btn"
      onclick={cycleTTL}
      title="Самоуничтожение"
      style:background={currentTTL > 0 ? '#c62828' : '#1a1a1a'}
      style:color={currentTTL > 0 ? '#fff' : '#888'}
    >
      {ttlLabels[ttlOptions.indexOf(currentTTL)]}
    </button>
    <button class="tool-btn" onclick={startAudioCall} title="Аудиозвонок">📞</button>
    <button class="tool-btn" onclick={startVideoCall} title="Видеозвонок">📹</button>
    <button class="tool-btn" onclick={ipfsAttach} title="Прикрепить файл (IPFS)">📎</button>
    <input type="file" id="ipfs-file-input" style="display:none" onchange={ipfsFileSelected(this)} />
    <button onclick={send}>➤</button>
  </div>
</div>

<style>
  .chat-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }
  .welcome {
    text-align: center;
    margin-top: 30vh;
    opacity: 0.4;
  }
  .welcome h3 {
    margin-bottom: 8px;
    font-size: 18px;
  }
  .welcome p {
    font-size: 13px;
    line-height: 1.5;
  }
  .typing {
    padding: 4px 16px;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.6);
    min-height: 20px;
    font-style: italic;
  }
  .reply-bar {
    padding: 8px 12px;
    background: rgba(20, 20, 20, 0.8);
    backdrop-filter: blur(10px);
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .reply-preview {
    flex: 1;
    font-size: 13px;
    color: rgba(255, 255, 255, 0.8);
    border-left: 3px solid #4fc3f7;
    padding-left: 10px;
  }
  .rp-from {
    color: #4fc3f7;
    font-size: 12px;
    font-weight: 500;
  }
  .rp-text {
    color: rgba(255, 255, 255, 0.7);
    margin-top: 2px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .reply-cancel {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.6);
    cursor: pointer;
    font-size: 20px;
    padding: 4px;
    transition: color 0.2s;
  }
  .reply-cancel:hover {
    color: #ffffff;
  }
  .input-bar {
    display: flex;
    padding: 12px 16px;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    gap: 10px;
    background: rgba(10, 10, 10, 0.8);
    backdrop-filter: blur(15px);
  }
  .input-bar input {
    flex: 1;
    padding: 12px 16px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 20px;
    color: #ffffff;
    font-size: 15px;
    outline: none;
    transition: all 0.2s;
  }
  .input-bar input::placeholder {
    color: rgba(255, 255, 255, 0.4);
  }
  .input-bar input:focus {
    border-color: #4fc3f7;
    background: rgba(255, 255, 255, 0.1);
    box-shadow: 0 0 10px rgba(79, 195, 247, 0.2);
  }
  .input-bar button {
    padding: 12px 20px;
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #ffffff;
    border: none;
    border-radius: 20px;
    cursor: pointer;
    font-size: 16px;
    font-weight: 500;
    transition: all 0.2s;
    box-shadow: 0 4px 15px rgba(30, 136, 229, 0.3);
  }
  .input-bar button:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 20px rgba(30, 136, 229, 0.5);
  }
  .tool-btn {
    min-width: 44px;
    font-size: 16px !important;
    background: rgba(255, 255, 255, 0.1) !important;
    border-radius: 50% !important;
    padding: 0 !important;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: none !important;
  }
  .tool-btn:hover {
    background: rgba(255, 255, 255, 0.2) !important;
  }
</style>
