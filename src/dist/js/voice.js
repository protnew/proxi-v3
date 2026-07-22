// Voice Messages module for Proxi Messenger
// Uses MediaRecorder API (audio/webm;codecs=opus)
// Max 2 min, sends via WebSocket binary frames

const VoiceMessages = (() => {
  let mediaRecorder = null;
  let chunks = [];
  let isRecording = false;
  let startTime = 0;
  let timerInterval = null;

  // Binary frame type byte for voice
  const BINARY_VOICE_TYPE = 0x02;

  // Start recording
  async function startRecording() {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      mediaRecorder = new MediaRecorder(stream, {
        mimeType: MediaRecorder.isTypeSupported('audio/webm;codecs=opus') 
          ? 'audio/webm;codecs=opus' 
          : 'audio/webm'
      });
      chunks = [];
      
      mediaRecorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunks.push(e.data);
      };

      mediaRecorder.start(1000); // collect chunks every second
      isRecording = true;
      startTime = Date.now();

      return true;
    } catch (e) {
      console.error('Microphone access denied:', e);
      return false;
    }
  }

  // Stop recording and return blob
  function stopRecording() {
    return new Promise((resolve) => {
      if (!mediaRecorder || !isRecording) {
        resolve(null);
        return;
      }

      mediaRecorder.onstop = () => {
        const duration = Math.round((Date.now() - startTime) / 1000);
        const blob = new Blob(chunks, { type: 'audio/webm' });
        
        // Stop all tracks
        mediaRecorder.stream.getTracks().forEach(t => t.stop());
        
        isRecording = false;
        mediaRecorder = null;
        chunks = [];

        resolve({ blob, duration });
      };

      mediaRecorder.stop();
    });
  }

  /**
   * Build a binary voice frame for WebSocket transmission.
   * Wire format: [0x02] [JSON metadata] [0x00] [audio bytes]
   * @param {Object} meta - { from, to, duration, ts }
   * @param {Blob} audioBlob - audio data blob
   * @returns {Promise<ArrayBuffer>} - binary frame data
   */
  async function buildBinaryVoiceFrame(meta, audioBlob) {
    const metaJson = new TextEncoder().encode(JSON.stringify(meta));
    const audioArrayBuffer = await audioBlob.arrayBuffer();
    const audioBytes = new Uint8Array(audioArrayBuffer);

    // Total: 1 (type) + metaJson.length + 1 (null) + audioBytes.length
    const frame = new Uint8Array(1 + metaJson.length + 1 + audioBytes.length);
    let offset = 0;
    frame[offset++] = BINARY_VOICE_TYPE;       // type byte
    frame.set(metaJson, offset);                // JSON metadata
    offset += metaJson.length;
    frame[offset++] = 0x00;                     // null separator
    frame.set(audioBytes, offset);              // audio data

    return frame.buffer;
  }

  /**
   * Parse a received binary voice frame.
   * @param {ArrayBuffer} buffer - the binary frame data
   * @returns {{ meta: Object, audioBlob: Blob }} parsed metadata and audio blob
   */
  function parseBinaryVoiceFrame(buffer) {
    const data = new Uint8Array(buffer);
    if (data.length < 2 || data[0] !== BINARY_VOICE_TYPE) {
      return null;
    }

    // Find null separator
    let nullIdx = -1;
    for (let i = 1; i < data.length; i++) {
      if (data[i] === 0x00) {
        nullIdx = i;
        break;
      }
    }
    if (nullIdx < 0) return null;

    // Parse metadata JSON
    const metaBytes = data.slice(1, nullIdx);
    const metaStr = new TextDecoder().decode(metaBytes);
    let meta;
    try {
      meta = JSON.parse(metaStr);
    } catch (e) {
      return null;
    }

    // Extract audio data
    const audioBytes = data.slice(nullIdx + 1);
    const audioBlob = new Blob([audioBytes], { type: 'audio/webm' });

    return { meta, audioBlob };
  }

  /**
   * Check if a binary message is a voice frame.
   * @param {ArrayBuffer|Blob} data 
   * @returns {boolean}
   */
  function isBinaryVoiceFrame(data) {
    if (data instanceof ArrayBuffer) {
      const arr = new Uint8Array(data);
      return arr.length > 0 && arr[0] === BINARY_VOICE_TYPE;
    }
    return false;
  }

  // Convert blob to base64 (legacy fallback)
  function blobToBase64(blob) {
    return new Promise((resolve) => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result.split(',')[1]);
      reader.readAsDataURL(blob);
    });
  }

  // Create audio element for playback from audio blob
  function createAudioPlayerFromBlob(audioBlob, duration) {
    const url = URL.createObjectURL(audioBlob);
    return `<div class="voice-msg">
      <audio controls src="${url}" style="height:32px;max-width:200px"></audio>
      <span style="font-size:10px;color:#666;margin-left:4px">${duration}s</span>
    </div>`;
  }

  // Create audio element for playback from base64 (legacy)
  function createAudioPlayer(base64Data, duration) {
    const url = 'data:audio/webm;base64,' + base64Data;
    return `<div class="voice-msg">
      <audio controls src="${url}" style="height:32px;max-width:200px"></audio>
      <span style="font-size:10px;color:#666;margin-left:4px">${duration}s</span>
    </div>`;
  }

  /**
   * Send voice message via binary WebSocket frame.
   * Falls back to base64 JSON if ws doesn't support binary.
   * @param {WebSocket} ws - open WebSocket connection
   * @param {Blob} audioBlob - recorded audio
   * @param {number} duration - duration in seconds
   * @param {string} from - sender user ID
   * @param {string} to - recipient or 'broadcast'
   * @returns {Promise<boolean>} true if sent successfully
   */
  async function sendVoiceBinary(ws, audioBlob, duration, from, to) {
    const meta = {
      from: from,
      to: to || 'broadcast',
      duration: duration,
      ts: Math.floor(Date.now() / 1000)
    };

    try {
      const frameBuffer = await buildBinaryVoiceFrame(meta, audioBlob);
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(frameBuffer);
        return true;
      }
    } catch (e) {
      console.warn('Binary voice send failed, falling back to base64:', e);
    }

    // Fallback: base64 JSON
    try {
      const b64 = await blobToBase64(audioBlob);
      if (ws && ws.readyState === WebSocket.OPEN) {
        const msg = {
          type: 'chat',
          from: from,
          to: to || 'broadcast',
          text: '🎤 Голосовое сообщение (' + duration + 'с)',
          ts: Math.floor(Date.now() / 1000),
          voiceData: b64,
          voiceDuration: duration
        };
        ws.send(JSON.stringify(msg));
        return true;
      }
    } catch (e) {
      console.error('Voice send failed completely:', e);
    }
    return false;
  }

  function getIsRecording() { return isRecording; }
  
  function getDuration() {
    return isRecording ? Math.round((Date.now() - startTime) / 1000) : 0;
  }

  return { 
    startRecording, 
    stopRecording, 
    blobToBase64, 
    createAudioPlayer, 
    createAudioPlayerFromBlob,
    getIsRecording, 
    getDuration,
    sendVoiceBinary,
    buildBinaryVoiceFrame,
    parseBinaryVoiceFrame,
    isBinaryVoiceFrame,
    BINARY_VOICE_TYPE
  };
})();
