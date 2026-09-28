<script lang="ts">
  import EmojiPicker from './EmojiPicker.svelte'
  let {
    inputText = $bindable(''),
    showEmoji = $bindable(false),
    isRecording = false,
    replyTo = null,
    editingMsg = null,
    uploading = false,
    uploadName = '',
    uploadProgress = 0,
    addEmoji,
    handleKey,
    sendMessage,
    startRecording,
    stopRecording,
    handleFileSelect,
    clearReply,
    clearEdit,
  }: {
    inputText: string
    showEmoji: boolean
    isRecording?: boolean
    replyTo?: { text: string } | null
    editingMsg?: { text: string } | null
    uploading?: boolean
    uploadName?: string
    uploadProgress?: number
    addEmoji: (e: string) => void
    handleKey: (e: KeyboardEvent) => void
    sendMessage: () => void
    startRecording: () => void
    stopRecording: () => void
    handleFileSelect: (e: Event) => void
    clearReply: () => void
    clearEdit: () => void
  } = $props()
</script>
{#if replyTo}
  <div class="reply-bar">
    <span>↩ {replyTo.text.slice(0, 50)}{replyTo.text.length > 50 ? '...' : ''}</span>
    <button onclick={clearReply}>✕</button>
  </div>
{/if}
{#if editingMsg}
  <div class="reply-bar editing">
    <span>✏️ Редактирование: {editingMsg.text.slice(0, 50)}</span>
    <button onclick={clearEdit}>✕</button>
  </div>
{/if}
<div class="input-area">
  <button class="ibtn" onclick={() => showEmoji = !showEmoji} aria-label="Смайлики" title="Смайлики">😊</button>
  <EmojiPicker {showEmoji} onSelect={addEmoji} />
  <button class="ibtn" onclick={() => document.getElementById('f-in')?.click()} aria-label="Файл" title="Файл">📎</button>
  <input id="f-in" type="file" hidden onchange={handleFileSelect} />
  <textarea
    placeholder={editingMsg ? 'Редактировать сообщение...' : 'Сообщение'}
    bind:value={inputText}
    onkeydown={handleKey}
    rows="1"
  ></textarea>
  {#if inputText.trim()}
    <button class="send-btn" onclick={sendMessage} aria-label="Отправить" title="Отправить">➤</button>
  {:else}
    <button class="mic-btn" class:rec={isRecording} aria-label="Голосовое" title="Голосовое"
      onmousedown={startRecording} onmouseup={stopRecording} onmouseleave={() => isRecording && stopRecording()}>
      {isRecording ? '⏺' : '🎤'}
    </button>
  {/if}
</div>
{#if uploading}
  <div class="upload-bar">
    <span>📤 {uploadName}</span>
    <div class="progress-track"><div class="progress-fill" style="width:{uploadProgress}%"></div></div>
    <span>{uploadProgress}%</span>
  </div>
{/if}
