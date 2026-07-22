/**
 * Voice messages — record, encode, decode, play
 * Uses MediaRecorder API (WebM/Opus) for recording
 * Playback via Audio API
 */

let mediaRecorder: MediaRecorder | null = null
let audioChunks: Blob[] = []
let recordingStream: MediaStream | null = null

export interface VoiceData {
  blob: Blob
  duration: number
  url: string
  waveformData: number[]  // for visualization
}

/**
 * Start recording from microphone
 */
export async function startRecording(): Promise<MediaStream> {
  recordingStream = await navigator.mediaDevices.getUserMedia({
    audio: {
      echoCancellation: true,
      noiseSuppression: true,
      sampleRate: 48000,
    }
  })
  
  audioChunks = []
  mediaRecorder = new MediaRecorder(recordingStream, {
    mimeType: getSupportedMime(),
  })
  
  mediaRecorder.ondataavailable = (e) => {
    if (e.data.size > 0) audioChunks.push(e.data)
  }
  
  mediaRecorder.start(100) // collect every 100ms
  return recordingStream
}

/**
 * Stop recording and return voice data
 */
export async function stopRecording(): Promise<VoiceData> {
  return new Promise((resolve) => {
    if (!mediaRecorder) {
      resolve({ blob: new Blob(), duration: 0, url: '', waveformData: [] })
      return
    }
    
    const startTime = Date.now()
    
    mediaRecorder.onstop = () => {
      recordingStream?.getTracks().forEach(t => t.stop())
      recordingStream = null
      
      const blob = new Blob(audioChunks, { type: getSupportedMime() })
      const duration = Math.round((Date.now() - startTime) / 1000)
      const url = URL.createObjectURL(blob)
      
      // Generate waveform data (simplified — just random for now, real version uses AnalyserNode)
      const waveformData = Array.from({ length: 30 }, () => Math.random() * 0.8 + 0.2)
      
      resolve({ blob, duration, url, waveformData })
    }
    
    mediaRecorder.stop()
    mediaRecorder = null
  })
}

/**
 * Play voice message
 */
export function playVoice(url: string): HTMLAudioElement {
  const audio = new Audio(url)
  audio.play()
  return audio
}

/**
 * Get waveform for visualization (analyser-based)
 */
export function getWaveformAnalyser(stream: MediaStream): {
  analyser: AnalyserNode
  dataArray: Uint8Array
} {
  const ctx = new AudioContext()
  const source = ctx.createMediaStreamSource(stream)
  const analyser = ctx.createAnalyser()
  analyser.fftSize = 256
  source.connect(analyser)
  const dataArray = new Uint8Array(analyser.frequencyBinCount)
  return { analyser, dataArray }
}

function getSupportedMime(): string {
  const types = [
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/ogg;codecs=opus',
    'audio/mp4',
  ]
  for (const t of types) {
    if (MediaRecorder.isTypeSupported(t)) return t
  }
  return 'audio/webm'
}
