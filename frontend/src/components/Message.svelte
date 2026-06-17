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
    padding: 12px 16px;
    border-radius: 18px;
    font-size: 15px;
    line-height: 1.4;
    word-wrap: break-word;
    box-shadow: 0 4px 15px rgba(0,0,0,0.2);
  }
  .msg.me .msg-bubble {
    background: linear-gradient(135deg, #1e88e5, #1565c0);
    color: #ffffff;
    border-bottom-right-radius: 4px;
  }
  .msg.other .msg-bubble {
    background: rgba(40, 40, 40, 0.8);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #ffffff;
    border-bottom-left-radius: 4px;
  }
  .msg.system .msg-bubble {
    background: transparent;
    color: rgba(255, 255, 255, 0.6);
    font-size: 12px;
    text-align: center;
    box-shadow: none;
  }
  .msg-time {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.5);
    margin-top: 4px;
  }
  .msg.me .msg-time {
    text-align: right;
  }
  .msg-sender {
    font-size: 12px;
    color: #4fc3f7;
    margin-bottom: 4px;
    font-weight: 500;
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
    background: rgba(30, 30, 30, 0.9);
    border: 1px solid rgba(255, 255, 255, 0.2);
    color: #ffffff;
    cursor: pointer;
    font-size: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }
  .msg-action-btn:hover {
    background: #1e88e5;
    border-color: #1e88e5;
  }
  .msg-reply {
    background: rgba(30, 136, 229, 0.1);
    border-left: 3px solid #1e88e5;
    padding: 6px 10px;
    margin-bottom: 6px;
    border-radius: 6px;
    font-size: 12px;
    cursor: pointer;
  }
  .mr-from {
    color: #4fc3f7;
    font-weight: 500;
  }
  .mr-text {
    color: rgba(255, 255, 255, 0.8);
    margin-top: 2px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .msg-fwd {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.6);
    margin-bottom: 4px;
    font-style: italic;
  }
  .ttl-indicator {
    font-size: 10px;
    color: #ff5252;
    margin-top: 4px;
    font-weight: bold;
  }
  .voice-msg {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
  }
</style>
