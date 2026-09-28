<script lang="ts">
  import { onMount } from 'svelte';
  import { initNostr, connectToPeer, sendChatMessage, startLocalVideo, stopLocalVideo, webrtcState, chatMessages, localStream, remoteStream, sendVpnRequest } from '../lib/webrtc_mvp';
  
  let myPk = '';
  let targetPk = '';
  let chatText = '';
  let vpnUrl = 'https://api.ipify.org';
  let vpnIframeUrl = '';
  let vpnProxyMode = false;
  
  let localVideoRef: HTMLVideoElement;
  let remoteVideoRef: HTMLVideoElement;

  onMount(async () => {
    myPk = await initNostr();
    
    // Register Service Worker Message Listener for VPN
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.addEventListener('message', (event) => {
            if (event.data && event.data.type === 'vpn_request') {
                sendVpnRequest(event.data);
            }
        });
    }

    const unsubLocal = localStream.subscribe(stream => {
        if (localVideoRef && stream) {
            localVideoRef.srcObject = stream;
        } else if (localVideoRef && !stream) {
            localVideoRef.srcObject = null;
        }
    });

    const unsubRemote = remoteStream.subscribe(stream => {
        if (remoteVideoRef && stream) {
            remoteVideoRef.srcObject = stream;
        } else if (remoteVideoRef && !stream) {
            remoteVideoRef.srcObject = null;
        }
    });

    return () => {
        unsubLocal();
        unsubRemote();
    };
  });

  function handleConnect() {
      if (targetPk) connectToPeer(targetPk);
  }

  function handleChatSend() {
      if (chatText.trim()) {
          sendChatMessage(chatText);
          chatText = '';
      }
  }

  function toggleCamera() {
      if ($localStream) {
          stopLocalVideo();
      } else {
          startLocalVideo();
      }
  }

  function loadVpnUrl() {
      // Set iframe src to use our service worker proxy prefix
      let cleanUrl = vpnUrl;
      if (!cleanUrl.startsWith('http')) cleanUrl = 'https://' + cleanUrl;
      vpnIframeUrl = `/vpn-proxy/` + cleanUrl;
  }
</script>

<div class="mvp-container">
    <h2>Proxi P2P MVP</h2>
    
    <div class="card signaling">
        <p><strong>My Nostr ID:</strong> {myPk}</p>
        <div class="connect-row">
            <input placeholder="Peer Nostr ID" bind:value={targetPk} />
            <button on:click={handleConnect}>Connect</button>
        </div>
        <p>Status: <strong>{$webrtcState}</strong></p>
    </div>

    <div class="columns">
        <!-- Chat Section -->
        <div class="card chat">
            <h3>P2P Chat</h3>
            <div class="chat-log">
                {#each $chatMessages as msg}
                    <div class="msg {msg.sender}">
                        {msg.text}
                    </div>
                {/each}
            </div>
            <div class="chat-input">
                <input bind:value={chatText} on:keydown={(e) => e.key === 'Enter' && handleChatSend()} placeholder="Type a message..." />
                <button on:click={handleChatSend}>Send</button>
            </div>
        </div>

        <!-- Video Call Section -->
        <div class="card video-call">
            <h3>P2P Call</h3>
            <button on:click={toggleCamera}>{$localStream ? 'Stop Camera' : 'Start Camera'}</button>
            <div class="video-grid">
                <video bind:this={localVideoRef} autoplay playsinline muted></video>
                <video bind:this={remoteVideoRef} autoplay playsinline></video>
            </div>
        </div>
    </div>

    <!-- VPN Section -->
    <div class="card vpn">
        <h3>VPN Tunnel (Service Worker Proxy)</h3>
        <p>Route requests through the connected peer's browser.</p>
        <div class="vpn-bar">
            <input bind:value={vpnUrl} placeholder="https://api.ipify.org" />
            <button on:click={loadVpnUrl}>Go (Proxied)</button>
        </div>
        <div class="vpn-browser">
            {#if vpnIframeUrl}
                <iframe src={vpnIframeUrl} title="VPN Browser" sandbox="allow-scripts allow-same-origin"></iframe>
            {:else}
                <div class="iframe-placeholder">Proxied content will appear here</div>
            {/if}
        </div>
    </div>
</div>

<style>
  .mvp-container { max-width: 1000px; margin: 0 auto; padding: 20px; font-family: var(--font-family); color: var(--text-primary); }
  .card { background: var(--bg-panel); padding: 15px; margin-bottom: 20px; border-radius: 12px; border: 1px solid var(--border-glass); backdrop-filter: blur(24px); -webkit-backdrop-filter: blur(24px); box-shadow: var(--shadow-glass); }
  .columns { display: flex; gap: 20px; }
  .columns > div { flex: 1; }
  
  input { padding: 10px; border-radius: 8px; border: 1px solid var(--border-strong); background: var(--bg-glass); color: var(--text-primary); width: 100%; margin-bottom: 10px; outline: none; transition: all 0.2s; }
  input:focus { border-color: var(--accent); background: var(--bg-body); box-shadow: 0 0 10px rgba(2, 136, 209, 0.2); }
  button { padding: 10px 16px; background: linear-gradient(135deg, var(--accent), var(--accent-hover)); color: #ffffff; border: none; border-radius: 8px; cursor: pointer; font-weight: 600; transition: all 0.2s; }
  button:hover { box-shadow: 0 4px 15px rgba(2, 136, 209, 0.3); transform: translateY(-1px); }
  
  .connect-row { display: flex; gap: 10px; }
  
  .chat-log { height: 200px; background: var(--bg-body); overflow-y: auto; padding: 10px; border-radius: 8px; margin-bottom: 10px; display: flex; flex-direction: column; gap: 5px; border: 1px solid var(--border-glass); }
  .msg { padding: 8px 12px; border-radius: 8px; max-width: 80%; font-size: 14px; }
  .msg.me { background: var(--accent); color: #ffffff; align-self: flex-end; }
  .msg.peer { background: var(--bg-glass); border: 1px solid var(--border-strong); color: var(--text-primary); align-self: flex-start; }
  .chat-input { display: flex; gap: 10px; }
  
  .video-grid { display: flex; gap: 10px; margin-top: 10px; }
  video { background: #000; width: 100%; height: 150px; border-radius: 8px; object-fit: cover; }
  
  .vpn-bar { display: flex; gap: 10px; margin-bottom: 10px; }
  .vpn-browser { height: 300px; background: var(--bg-body); border-radius: 8px; border: 1px solid var(--border-glass); overflow: hidden; }
  iframe { width: 100%; height: 100%; border: none; }
  .iframe-placeholder { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-muted); font-size: 14px; }
</style>
