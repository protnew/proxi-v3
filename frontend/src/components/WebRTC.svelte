<script>
  import { onMount } from 'svelte';
  import * as WebRTCCall from '../lib/webrtc.js';

  let { showPanel = false } = $props();
  let callStatus = $state('Звоним...');
  let muted = $state(false);
  let remoteVideoEl = $state();
  let localVideoEl = $state();

  onMount(() => {
    WebRTCCall.setOnStateChange((state) => {
      if (state === 'idle') {
        showPanel = false;
      } else {
        callStatus =
          state === 'connecting' ? 'Звоним...' : state === 'connected' ? 'Подключён' : state;
      }
    });

    WebRTCCall.setOnRemoteStream((stream) => {
      if (remoteVideoEl) remoteVideoEl.srcObject = stream;
      if (localVideoEl && WebRTCCall.getLocalStream()) {
        localVideoEl.srcObject = WebRTCCall.getLocalStream();
      }
    });
  });

  function hangup() {
    WebRTCCall.hangup();
  }

  function toggleMute() {
    const unmuted = WebRTCCall.toggleMute();
    muted = !unmuted;
  }
</script>

{#if showPanel}
  <div class="call-panel">
    <video bind:this={remoteVideoEl} autoplay playsinline class="remote-video"></video>
    <video bind:this={localVideoEl} autoplay playsinline muted class="local-video"></video>
    <div class="call-controls">
      <button class="call-btn" onclick={toggleMute}>{muted ? '🔇' : '🎤'}</button>
      <button class="call-btn hangup" onclick={hangup}>📵</button>
    </div>
    <div class="call-status">{callStatus}</div>
  </div>
{/if}

<style>
  .call-panel {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 100;
    background: #0a0a0a;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }
  .remote-video {
    width: 100%;
    max-height: 70vh;
    background: #000;
    border-radius: 8px;
  }
  .local-video {
    position: absolute;
    top: 10px;
    right: 10px;
    width: 120px;
    height: 90px;
    border-radius: 6px;
    border: 1px solid #333;
  }
  .call-controls {
    margin-top: 16px;
    display: flex;
    gap: 12px;
  }
  .call-btn {
    background: #1a1a1a;
    font-size: 20px;
    width: 48px;
    height: 48px;
    border-radius: 50%;
    border: none;
    color: #e0e0e0;
    cursor: pointer;
  }
  .hangup {
    background: #c0392b;
    color: white;
  }
  .call-status {
    margin-top: 8px;
    color: #888;
    font-size: 14px;
  }
</style>
