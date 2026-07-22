// Voice Messages module
// Uses MediaRecorder API (audio/webm;codecs=opus)
// Max 2 min, sends via WebSocket binary frames

const BINARY_VOICE_TYPE = 0x02;

let mediaRecorder = null;
let chunks = [];
let isRecording = false;
let startTime = 0;

export function getIsRecording() {
  return isRecording;
}

export function getDuration() {
  return isRecording ? Math.round((Date.now() - startTime) / 1000) : 0;
}

export async function startRecording() {
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    mediaRecorder = new MediaRecorder(stream, {
      mimeType: MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
        ? 'audio/webm;codecs=opus'
        : 'audio/webm',
    });
    chunks = [];

    mediaRecorder.ondataavailable = (e) => {
      if (e.data.size > 0) chunks.push(e.data);
    };

    mediaRecorder.start(1000);
    isRecording = true;
    startTime = Date.now();

    return true;
  } catch (e) {
    console.error('Microphone access denied:', e);
    return false;
  }
}

export function stopRecording() {
  return new Promise((resolve) => {
    if (!mediaRecorder || !isRecording) {
      resolve(null);
      return;
    }

    mediaRecorder.onstop = () => {
      const duration = Math.round((Date.now() - startTime) / 1000);
      const blob = new Blob(chunks, { type: 'audio/webm' });

      mediaRecorder.stream.getTracks().forEach((t) => t.stop());

      isRecording = false;
      mediaRecorder = null;
      chunks = [];

      resolve({ blob, duration });
    };

    mediaRecorder.stop();
  });
}

async function buildBinaryVoiceFrame(meta, audioBlob) {
  const metaJson = new TextEncoder().encode(JSON.stringify(meta));
  const audioArrayBuffer = await audioBlob.arrayBuffer();
  const audioBytes = new Uint8Array(audioArrayBuffer);

  const frame = new Uint8Array(1 + metaJson.length + 1 + audioBytes.length);
  let offset = 0;
  frame[offset++] = BINARY_VOICE_TYPE;
  frame.set(metaJson, offset);
  offset += metaJson.length;
  frame[offset++] = 0x00;
  frame.set(audioBytes, offset);

  return frame.buffer;
}

export function parseBinaryVoiceFrame(buffer) {
  const data = new Uint8Array(buffer);
  if (data.length < 2 || data[0] !== BINARY_VOICE_TYPE) {
    return null;
  }

  let nullIdx = -1;
  for (let i = 1; i < data.length; i++) {
    if (data[i] === 0x00) {
      nullIdx = i;
      break;
    }
  }
  if (nullIdx < 0) return null;

  const metaBytes = data.slice(1, nullIdx);
  const metaStr = new TextDecoder().decode(metaBytes);
  let meta;
  try {
    meta = JSON.parse(metaStr);
  } catch (e) {
    return null;
  }

  const audioBytes = data.slice(nullIdx + 1);
  const audioBlob = new Blob([audioBytes], { type: 'audio/webm' });

  return { meta, audioBlob };
}

function blobToBase64(blob) {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(reader.result.split(',')[1]);
    reader.readAsDataURL(blob);
  });
}

export async function sendVoiceBinary(ws, audioBlob, duration, from, to) {
  const meta = {
    from: from,
    to: to || 'broadcast',
    duration: duration,
    ts: Math.floor(Date.now() / 1000),
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
        voiceDuration: duration,
      };
      ws.send(JSON.stringify(msg));
      return true;
    }
  } catch (e) {
    console.error('Voice send failed completely:', e);
  }
  return false;
}
