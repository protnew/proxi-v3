<script>
  import { onMount } from 'svelte';
  import { messages, myId, showToast, escHtml } from '../lib/stores.js';
  import { icons } from '../lib/icons.js';
  import { sendMessage, getWS } from '../lib/ws.js';
  import { createWebSocket, getMessages as apiGetMessages, apiFetch } from '../lib/api.js';
  import * as VoiceMessages from '../lib/voice.js';
  import * as OfflineStorage from '../lib/offline.js';
  import * as WebRTCCall from '../lib/webrtc.js';
  import Message from './Message.svelte';
  import VoiceRecorder from './VoiceRecorder.svelte';
  import WebRTC from './WebRTC.svelte';
  import SkeletonMessage from './SkeletonMessage.svelte';
  import AttachmentPicker from './AttachmentPicker.svelte';

  let loading = $state(true);
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
              isE2E: msg.is_e2e || false,
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

  function handleAttachmentUpload(e) {
    const { cid, name, size } = e.detail;
    const sizeKB = (size / 1024).toFixed(1);
    const text = `📎 ${name} (${sizeKB} KB, CID: ${cid})`;
    sendMessage(text);
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
    apiGetMessages('broadcast', 50).then(() => {
      loading = false;
    }).catch(() => {
      loading = false;
      showToast('❌ Ошибка загрузки сообщений');
    });
  });
</script>

<div class="chat-container">
  {#if showCallPanel}
    <WebRTC showPanel={showCallPanel} />
  {/if}

  <div class="messages" bind:this={msgListEl}>
    {#if loading}
      <SkeletonMessage />
      <SkeletonMessage />
      <SkeletonMessage />
    {:else if $messages.length === 0}
      <div class="welcome">
        <div class="welcome-icon">{@html icons.chat}</div>
        <h3>Мессенджер</h3>
        <p>E2E-протокол · P2P · Без цензуры<br />Открой с другого устройства чтобы начать</p>
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
    <button class="tool-btn" onclick={startAudioCall} title="Аудиозвонок">{@html icons.audio}</button>
    <button class="tool-btn" onclick={startVideoCall} title="Видеозвонок">{@html icons.video}</button>
    <AttachmentPicker on:upload={handleAttachmentUpload} />
    <button class="send-btn" onclick={send} title="Отправить">{@html icons.send}</button>
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
    color: var(--text-primary);
  }
  .welcome p {
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
  }
  .typing {
    padding: 4px 16px;
    font-size: 12px;
    color: var(--text-secondary);
    min-height: 20px;
    font-style: italic;
  }
  .reply-bar {
    padding: 8px 12px;
    background: var(--bg-panel);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border-top: 1px solid var(--border-glass);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .reply-preview {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary);
    border-left: 3px solid var(--accent);
    padding-left: 10px;
  }
  .rp-from {
    color: var(--accent);
    font-size: 12px;
    font-weight: 600;
  }
  .rp-text {
    color: var(--text-secondary);
    margin-top: 2px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .reply-cancel {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 20px;
    padding: 4px;
    transition: color 0.2s;
  }
  .reply-cancel:hover {
    color: var(--error);
  }
  .input-bar {
    display: flex;
    padding: 12px 16px;
    border-top: 1px solid var(--border-glass);
    gap: 10px;
    background: var(--bg-panel);
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
  }
  .input-bar input {
    flex: 1;
    padding: 12px 16px;
    background: var(--bg-glass);
    border: 1px solid var(--border-strong);
    border-radius: 20px;
    color: var(--text-primary);
    font-size: 15px;
    outline: none;
    transition: all 0.2s;
  }
  .input-bar input::placeholder {
    color: var(--text-muted);
  }
  .input-bar input:focus {
    border-color: var(--accent);
    background: var(--bg-glass);
    box-shadow: 0 0 12px var(--selection-bg);
  }
  .input-bar button {
    padding: 12px 20px;
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: var(--text-primary);
    border: none;
    border-radius: 20px;
    cursor: pointer;
    font-size: 16px;
    font-weight: 600;
    transition: all 0.2s;
    box-shadow: 0 4px 15px var(--selection-bg);
  }
  .input-bar button:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 20px var(--selection-bg);
  }
  .tool-btn {
    min-width: 44px;
    font-size: 16px !important;
    background: var(--bg-glass) !important;
    border: 1px solid var(--border-glass) !important;
    border-radius: 50% !important;
    padding: 0 !important;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: none !important;
    transition: all 0.2s;
  }
  .tool-btn:hover {
    background: var(--border-strong) !important;
  }
</style>
