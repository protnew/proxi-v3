// Voice Messages module for Proxi Messenger
// Uses MediaRecorder API (audio/webm;codecs=opus)
// Max 2 min, sends via WebSocket binary frames

const VoiceMessages = (() => {
  let mediaRecorder = null;
  let chunks = [];
  let isRecording = false;
  let startTime = 0;
  let timerInterval = null;

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

  // Convert blob to base64
  function blobToBase64(blob) {
    return new Promise((resolve) => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result.split(',')[1]);
      reader.readAsDataURL(blob);
    });
  }

  // Create audio element for playback
  function createAudioPlayer(base64Data, duration) {
    const url = 'data:audio/webm;base64,' + base64Data;
    return `<div class="voice-msg">
      <audio controls src="${url}" style="height:32px;max-width:200px"></audio>
      <span style="font-size:10px;color:#666;margin-left:4px">${duration}s</span>
    </div>`;
  }

  function getIsRecording() { return isRecording; }
  
  function getDuration() {
    return isRecording ? Math.round((Date.now() - startTime) / 1000) : 0;
  }

  return { startRecording, stopRecording, blobToBase64, createAudioPlayer, getIsRecording, getDuration };
})();
