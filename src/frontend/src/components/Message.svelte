<script>
  import { escHtml } from '../lib/stores.js';

  let { msg, startReply, forwardMsg } = $props();

  function onReply() {
    startReply(msg.id, msg.from, msg.text?.substring(0, 60) || '');
  }

  function onForward() {
    forwardMsg(msg.from, msg.text);
  }
</script>

{#if msg.isSystem}
  <div class="msg system">
    <div class="msg-bubble">{msg.text}</div>
  </div>
{:else if msg.isVoice}
  <div class="msg" class:me={msg.isMe} class:other={!msg.isMe}>
    {#if !msg.isMe}
      <div class="msg-sender">{msg.from.substring(0, 12)}...</div>
    {/if}
    <div class="msg-bubble">
      <div class="voice-msg">
        <audio
          controls
          src={msg.audioUrl || (msg.audioBlob ? URL.createObjectURL(msg.audioBlob) : '')}
          style="height:32px;max-width:200px"
        ></audio>
        {#if msg.type === 'audio'}
          <span style="font-size:10px;color:var(--text-muted);margin-left:4px">{msg.duration}с</span>
        {/if}
      </div>
    </div>
    <div class="msg-time">{msg.time}</div>
  </div>
{:else}
  <div class="msg" class:me={msg.isMe} class:other={!msg.isMe} data-msg-id={msg.id}>
    {#if !msg.isMe}
      <div class="msg-sender">{msg.from.substring(0, 12)}...</div>
    {/if}

    <div class="msg-actions">
      <button class="msg-action-btn" onclick={onReply} title="Ответить">↩</button>
      <button class="msg-action-btn" onclick={onForward} title="Переслать">↗</button>
    </div>

    {#if msg.replyTo}
      <div class="msg-reply">
        <div class="mr-from">{msg.replyToFrom?.substring(0, 12) || ''}...</div>
        <div class="mr-text">{msg.replyToText || '...'}</div>
      </div>
    {/if}

    {#if msg.forwardedFrom}
      <div class="msg-fwd">↗ Переслано от {msg.forwardedFrom?.substring(0, 12)}...</div>
    {/if}

    <div class="msg-bubble">{msg.text}</div>

    {#if msg.ttl > 0}
      <div class="ttl-indicator">
        💣 Исчезнет через ~{msg.ttl < 3600 ? Math.round(msg.ttl / 60) + 'м' : Math.round(msg.ttl / 3600) + 'ч'}
      </div>
    {/if}

    <div class="msg-time">
      {#if msg.isE2E}
        <span class="e2e-lock" title="Зашифровано (E2E)">🔒</span>
      {/if}
      {msg.time}
    </div>
  </div>
{/if}

<style>
  .msg {
    margin-bottom: 12px;
    max-width: 80%;
    position: relative;
  }
  .msg.me {
    margin-left: auto;
  }
  .msg-bubble {
    max-width: 75%;
    padding: 10px 14px;
    border-radius: 18px;
    font-size: 15px;
    line-height: 1.4;
    word-wrap: break-word;
    box-shadow: var(--shadow-glass);
  }
  .msg.me .msg-bubble {
    background: linear-gradient(135deg, var(--accent), var(--accent-hover));
    color: #ffffff; /* User bubble always white */
    border-bottom-right-radius: 4px;
  }
  .msg.other .msg-bubble {
    background: var(--bg-panel);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border: 1px solid var(--border-glass);
    color: var(--text-primary);
    border-bottom-left-radius: 4px;
  }
  .msg.system .msg-bubble {
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    text-align: center;
    box-shadow: none;
  }
  .msg-time {
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 4px;
  }
  .msg.me .msg-time {
    text-align: right;
  }
  .msg-sender {
    font-size: 12px;
    color: var(--accent);
    margin-bottom: 4px;
    font-weight: 600;
  }
  .msg-actions {
    position: absolute;
    right: -12px;
    top: -12px;
    display: none;
    gap: 6px;
  }
  .msg:hover .msg-actions {
    display: flex;
  }
  .msg-action-btn {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--bg-body);
    border: 1px solid var(--border-glass);
    color: var(--text-primary);
    cursor: pointer;
    font-size: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }
  .msg-action-btn:hover {
    background: var(--accent);
    border-color: var(--accent);
    color: #ffffff;
  }
  .msg-reply {
    background: var(--bg-glass);
    border-left: 3px solid var(--accent);
    padding: 6px 10px;
    margin-bottom: 6px;
    border-radius: 6px;
    font-size: 12px;
    cursor: pointer;
  }
  .mr-from {
    color: var(--accent);
    font-weight: 600;
  }
  .mr-text {
    color: var(--text-secondary);
    margin-top: 2px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .msg-fwd {
    font-size: 11px;
    color: var(--text-muted);
    margin-bottom: 4px;
    font-style: italic;
  }
  .ttl-indicator {
    font-size: 10px;
    color: var(--error);
    margin-top: 4px;
    font-weight: 700;
  }
  .voice-msg {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
  }
  .e2e-lock {
    font-size: 10px;
    margin-right: 4px;
    opacity: 0.8;
  }
</style>
