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
        <span style="font-size:10px;color:#666;margin-left:4px">{msg.duration}с</span>
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

    <div class="msg-time">{msg.time}</div>
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
    padding: 10px 14px;
    border-radius: 14px;
    font-size: 14px;
    line-height: 1.4;
    word-wrap: break-word;
  }
  .msg.me .msg-bubble {
    background: #1e88e5;
    color: #fff;
    border-bottom-right-radius: 4px;
  }
  .msg.other .msg-bubble {
    background: #1a1a1a;
    border: 1px solid #222;
    border-bottom-left-radius: 4px;
  }
  .msg.system .msg-bubble {
    background: transparent;
    color: #555;
    font-size: 12px;
    text-align: center;
  }
  .msg-time {
    font-size: 10px;
    color: #555;
    margin-top: 4px;
  }
  .msg.me .msg-time {
    text-align: right;
  }
  .msg-sender {
    font-size: 11px;
    color: #1e88e5;
    margin-bottom: 2px;
  }
  .msg-actions {
    position: absolute;
    right: -8px;
    top: -8px;
    display: none;
    gap: 4px;
  }
  .msg:hover .msg-actions {
    display: flex;
  }
  .msg-action-btn {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: #1a1a1a;
    border: 1px solid #333;
    color: #888;
    cursor: pointer;
    font-size: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .msg-action-btn:hover {
    background: #1e88e5;
    color: #fff;
    border-color: #1e88e5;
  }
  .msg-reply {
    background: #0d1520;
    border-left: 2px solid #1e88e5;
    padding: 4px 8px;
    margin-bottom: 4px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
  }
  .mr-from {
    color: #1e88e5;
  }
  .mr-text {
    color: #666;
    margin-top: 1px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .msg-fwd {
    font-size: 10px;
    color: #666;
    margin-bottom: 2px;
    font-style: italic;
  }
  .ttl-indicator {
    font-size: 9px;
    color: #c62828;
    margin-top: 2px;
  }
  .voice-msg {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 0;
  }
</style>
