<script lang="ts">
  import type { Message } from '../stores/messenger'
  let {
    currentChat,
    messagesEl = $bindable(),
    isMine,
    getReplyText,
    getName,
    formatTime,
    formatSize,
    getDate,
    onContext,
    downloadFile,
    doReact,
    handleFileSelect,
  }: {
    currentChat: { messages: Message[] }
    messagesEl?: HTMLDivElement
    isMine: (msg: Message) => boolean
    getReplyText: (id: string | undefined) => string
    getName: (id: string) => string
    formatTime: (ts: number) => string
    formatSize: (n: number) => string
    getDate: (ts: number) => string
    onContext: (e: MouseEvent, msg: Message) => void
    downloadFile: (msg: Message) => void
    doReact: (msg: Message, emoji: string) => void
    handleFileSelect: (e: any) => void
  } = $props()
</script>
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
          {#if isMine(msg)}
            <span class="check" title={msg.deliveryStatus || (msg.read ? 'read' : 'sent')}>
              {#if msg.deliveryStatus === 'pending'}⏳
              {:else if msg.deliveryStatus === 'failed'}⚠
              {:else if msg.deliveryStatus === 'delivered' || msg.read}✓✓
              {:else if msg.deliveryStatus === 'sent'}✓
              {:else}⏳{/if}
            </span>
          {/if}
        </div>
      </div>
    </div>
  {/each}
  {#if currentChat.messages.length === 0}
    <div class="no-msg">
      <p>Напишите первое сообщение</p>
      <p class="hint">Если друг сейчас не в сети, оно уйдёт позже</p>
    </div>
  {/if}
</div>
