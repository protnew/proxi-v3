<script>
  import { onMount } from 'svelte';
  import { showToast, myId } from '../lib/stores.js';
  import { getWS } from '../lib/ws.js';
  import * as VoiceMessages from '../lib/voice.js';

  let isRecording = $state(false);
  let btnText = $state('🎤');

  async function toggleVoice() {
    if (isRecording) {
      const result = await VoiceMessages.stopRecording();
      if (result && result.blob) {
        if (result.duration > 120) {
          showToast('❌ Голосовое сообщение максимум 2 минуты');
          return;
        }
        const ws = getWS();
        const id = $myId;
        const sent = await VoiceMessages.sendVoiceBinary(ws, result.blob, result.duration, id, 'broadcast');
        if (sent) {
          showToast('🎤 Голосовое отправлено');
        } else {
          showToast('❌ Не удалось отправить голосовое');
        }
      }
      isRecording = false;
      btnText = '🎤';
    } else {
      const ok = await VoiceMessages.startRecording();
      if (ok) {
        isRecording = true;
        btnText = '⏹';
        showToast('🎤 Запись...');
      } else {
        showToast('❌ Нет доступа к микрофону');
      }
    }
  }
</script>

<button
  class="voice-btn"
  onclick={toggleVoice}
  title="Голосовое сообщение"
  style:background={isRecording ? '#c62828' : '#1a1a1a'}
  style:color={isRecording ? '#fff' : '#888'}
>
  {btnText}
</button>

<style>
  .voice-btn {
    min-width: 42px;
    font-size: 14px;
    background: #1a1a1a;
    color: #888;
    padding: 12px 20px;
    border: none;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .voice-btn:hover {
    background: #222;
  }
</style>
