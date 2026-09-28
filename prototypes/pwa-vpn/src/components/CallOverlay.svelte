<script lang="ts">
  import { getCallState, getLocalStream, getRemoteStream, acceptCall, rejectCall, endCall, setOnCallStateChange } from '../lib/calls'

  let state = $state<'idle'|'ringing'|'connecting'|'connected'|'ended'>('idle')
  let fromPeer = $state('')
  let remoteAudio: HTMLAudioElement | undefined = $state()

  setOnCallStateChange((s) => { state = s })

  function doAccept() {
    const offer = (window as any).__pendingCallOffer
    if (offer) {
      acceptCall(offer.from, offer.sdp)
      fromPeer = offer.from
    }
  }

  function doReject() {
    const offer = (window as any).__pendingCallOffer
    if (offer) rejectCall(offer.from)
  }

  function doEnd() {
    endCall()
  }

  // Bind remote audio stream
  $effect(() => {
    const rs = getRemoteStream()
    if (rs && remoteAudio) {
      remoteAudio.srcObject = rs
    }
  })
</script>

{#if state !== 'idle'}
  <div class="call-overlay">
    <div class="call-card">
      {#if state === 'ringing'}
        <div class="ring-anim">📞</div>
        <h3>Входящий звонок</h3>
        <p class="peer">{fromPeer.slice(0, 16)}...</p>
        <div class="call-btns">
          <button class="accept" onclick={doAccept}>✅ Принять</button>
          <button class="reject" onclick={doReject}>❌ Отклонить</button>
        </div>
      {:else if state === 'connecting'}
        <div class="spinner-c">📞</div>
        <h3>Подключение...</h3>
        <button class="reject" onclick={doEnd}>❌ Отменить</button>
      {:else if state === 'connected'}
        <div class="connected-icon">🎙️</div>
        <h3>Разговор</h3>
        <p class="duration">00:00</p>
        <audio bind:this={remoteAudio} autoplay></audio>
        <button class="reject" onclick={doEnd}>📵 Завершить</button>
      {:else if state === 'ended'}
        <div>📵</div>
        <h3>Звонок завершён</h3>
        <button onclick={() => state = 'idle'}>Закрыть</button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .call-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.8); z-index: 300; display: flex; align-items: center; justify-content: center; }
  .call-card { background: var(--bg-tertiary); border-radius: 16px; padding: 32px; text-align: center; min-width: 300px; }
  h3 { margin: 12px 0 8px; }
  .peer { font-size: 12px; color: var(--text-muted); }
  .ring-anim { font-size: 48px; animation: pulse 1s infinite; }
  @keyframes pulse { 0%,100% { transform: scale(1); } 50% { transform: scale(1.1); } }
  .spinner-c { font-size: 48px; animation: spin 2s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .connected-icon { font-size: 48px; }
  .duration { color: var(--success); font-size: 18px; font-family: monospace; }
  .call-btns { display: flex; gap: 16px; justify-content: center; margin-top: 16px; }
  button { padding: 12px 24px; border: none; border-radius: 8px; font-size: 14px; cursor: pointer; }
  .accept { background: color-mix(in srgb, var(--success) 22%, var(--bg)); color: var(--success); }
  .reject { background: color-mix(in srgb, var(--danger) 22%, var(--bg)); color: var(--danger); margin-top: 12px; }
</style>
