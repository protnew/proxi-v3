// WebRTC Audio/Video Calls for Proxi Messenger
// Uses RTCPeerConnection for P2P calls, WebSocket for signaling

const WebRTCCall = (() => {
  const ICE_SERVERS = [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'stun:stun2.l.google.com:19302' }
  ];

  let peerConnection = null;
  let localStream = null;
  let remoteStream = null;
  let callState = 'idle'; // idle, ringing, connecting, connected
  let currentPeer = null;
  let ws = null; // reference to chat WebSocket

  // UI callbacks
  let onStateChange = null;
  let onRemoteStream = null;
  let onIncomingCall = null;

  function setState(state) {
    callState = state;
    if (onStateChange) onStateChange(state);
  }

  function setWS(websocket) {
    ws = websocket;
  }

  async function initCall(peerId, video = false) {
    if (callState !== 'idle') {
      console.warn('[webrtc] call already in progress');
      return;
    }

    currentPeer = peerId;
    setState('connecting');

    try {
      localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: video ? { width: 640, height: 480 } : false
      });

      peerConnection = createPeerConnection();

      localStream.getTracks().forEach(track => {
        peerConnection.addTrack(track, localStream);
      });

      const offer = await peerConnection.createOffer({
        offerToReceiveAudio: true,
        offerToReceiveVideo: video
      });

      await peerConnection.setLocalDescription(offer);

      // Send offer via signaling (WebSocket)
      sendSignal({
        type: 'call-offer',
        to: peerId,
        sdp: peerConnection.localDescription
      });

    } catch (err) {
      console.error('[webrtc] initCall error:', err);
      setState('idle');
      cleanup();
    }
  }

  function createPeerConnection() {
    const pc = new RTCPeerConnection({
      iceServers: ICE_SERVERS
    });

    pc.onicecandidate = (event) => {
      if (event.candidate) {
        sendSignal({
          type: 'ice-candidate',
          to: currentPeer,
          candidate: event.candidate
        });
      }
    };

    pc.ontrack = (event) => {
      remoteStream = event.streams[0];
      if (onRemoteStream) onRemoteStream(remoteStream);
    };

    pc.onconnectionstatechange = () => {
      switch (pc.connectionState) {
        case 'connected':
          setState('connected');
          break;
        case 'disconnected':
        case 'failed':
        case 'closed':
          hangup();
          break;
      }
    };

    pc.oniceconnectionstatechange = () => {
      if (pc.iceConnectionState === 'failed') {
        console.warn('[webrtc] ICE connection failed');
        hangup();
      }
    };

    return pc;
  }

  async function handleSignal(signal) {
    if (signal.type === 'call-offer' && signal.from) {
      // Incoming call
      currentPeer = signal.from;
      if (onIncomingCall) {
        onIncomingCall(signal.from, signal.sdp);
      }
      // Auto-accept for now — store offer for later
      window._pendingOffer = signal;

    } else if (signal.type === 'call-answer' && signal.sdp) {
      // Remote peer accepted our call
      if (peerConnection) {
        await peerConnection.setRemoteDescription(new RTCSessionDescription(signal.sdp));
      }

    } else if (signal.type === 'ice-candidate' && signal.candidate) {
      if (peerConnection) {
        try {
          await peerConnection.addIceCandidate(new RTCIceCandidate(signal.candidate));
        } catch (e) {
          console.warn('[webrtc] addIceCandidate error:', e);
        }
      }
    }
  }

  async function acceptCall(video = false) {
    const offer = window._pendingOffer;
    if (!offer) return;

    setState('connecting');

    try {
      localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: video ? { width: 640, height: 480 } : false
      });

      peerConnection = createPeerConnection();
      localStream.getTracks().forEach(track => {
        peerConnection.addTrack(track, localStream);
      });

      await peerConnection.setRemoteDescription(new RTCSessionDescription(offer.sdp));

      const answer = await peerConnection.createAnswer();
      await peerConnection.setLocalDescription(answer);

      sendSignal({
        type: 'call-answer',
        to: offer.from,
        sdp: peerConnection.localDescription
      });

      window._pendingOffer = null;

    } catch (err) {
      console.error('[webrtc] acceptCall error:', err);
      hangup();
    }
  }

  function rejectCall() {
    if (window._pendingOffer) {
      sendSignal({
        type: 'call-reject',
        to: window._pendingOffer.from
      });
      window._pendingOffer = null;
    }
    setState('idle');
  }

  function hangup() {
    if (currentPeer && callState !== 'idle') {
      sendSignal({
        type: 'call-hangup',
        to: currentPeer
      });
    }
    cleanup();
    setState('idle');
  }

  function cleanup() {
    if (localStream) {
      localStream.getTracks().forEach(track => track.stop());
      localStream = null;
    }
    if (peerConnection) {
      peerConnection.close();
      peerConnection = null;
    }
    remoteStream = null;
    currentPeer = null;
    window._pendingOffer = null;
  }

  function sendSignal(data) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'webrtc-signal',
        from: window.myId,
        to: data.to,
        signalType: data.type,
        sdp: data.sdp || null,
        candidate: data.candidate || null
      }));
    }
  }

  function getStats() {
    if (!peerConnection) return null;
    return {
      state: callState,
      peer: currentPeer,
      connectionState: peerConnection.connectionState,
      iceState: peerConnection.iceConnectionState
    };
  }

  // Toggle mute
  function toggleMute() {
    if (localStream) {
      const audioTrack = localStream.getAudioTracks()[0];
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled;
        return audioTrack.enabled;
      }
    }
    return null;
  }

  // Toggle video
  function toggleVideo() {
    if (localStream) {
      const videoTrack = localStream.getVideoTracks()[0];
      if (videoTrack) {
        videoTrack.enabled = !videoTrack.enabled;
        return videoTrack.enabled;
      }
    }
    return null;
  }

  return {
    setWS,
    initCall,
    acceptCall,
    rejectCall,
    hangup,
    handleSignal,
    toggleMute,
    toggleVideo,
    getStats,
    get state() { return callState; },
    get localStream() { return localStream; },
    get remoteStream() { return remoteStream; },
    set onStateChange(cb) { onStateChange = cb; },
    set onRemoteStream(cb) { onRemoteStream = cb; },
    set onIncomingCall(cb) { onIncomingCall = cb; }
  };
})();
