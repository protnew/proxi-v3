/**
 * Sound effects — programmatic WAV beeps, no external files
 * AudioContext created on first user interaction (browser policy)
 */

let audioCtx: AudioContext | null = null
let incomingBuffer: AudioBuffer | null = null
let outgoingBuffer: AudioBuffer | null = null

function ensureContext(): AudioContext | null {
  if (audioCtx) return audioCtx
  try {
    audioCtx = new AudioContext()
    return audioCtx
  } catch {
    return null
  }
}

function generateBeep(freq: number, duration: number, volume: number, type: 'incoming' | 'outgoing' | 'ring'): AudioBuffer | null {
  const ctx = ensureContext()
  if (!ctx) return null
  const sampleRate = ctx.sampleRate
  const numSamples = Math.floor(sampleRate * duration)
  const buffer = ctx.createBuffer(1, numSamples, sampleRate)
  const data = buffer.getChannelData(0)
  
  for (let i = 0; i < numSamples; i++) {
    const t = i / sampleRate
    const attack = 0.02
    const releaseTime = duration - 0.05 // Последние 50мс - полный fade to zero
    let envelope = 1
    
    if (t < attack) {
      envelope = t / attack
    } else if (t > releaseTime) {
      // Плавный уход в абсолютный ноль
      envelope = Math.exp(-4 * (releaseTime - attack) / duration) * (duration - t) / 0.05
    } else {
      envelope = Math.exp(-4 * (t - attack) / duration)
    }

    if (type === 'incoming') {
      const f1 = Math.sin(2 * Math.PI * freq * t)
      const f2 = Math.sin(2 * Math.PI * (freq * 1.25) * t)
      const f3 = Math.sin(2 * Math.PI * (freq * 1.5) * t)
      data[i] = (f1 + f2 * 0.6 + f3 * 0.4) * volume * envelope
    } else {
      const f1 = Math.sin(2 * Math.PI * freq * t)
      data[i] = f1 * volume * envelope
    }
  }
  return buffer
}

export function initSounds() {
  // Pre-generate buffers (AudioContext lazy-created on first call)
  // Actual init deferred to first user interaction
  console.log('[sounds] Ready (buffers will generate on first play)')
}

function playBuffer(buffer: AudioBuffer | null) {
  if (!buffer) return
  const ctx = ensureContext()
  if (!ctx) return
  // Resume if suspended (autoplay policy)
  if (ctx.state === 'suspended') ctx.resume()
  const source = ctx.createBufferSource()
  source.buffer = buffer
  source.connect(ctx.destination)
  source.start()
}

export function playIncoming() {
  // Мягкий аккорд от Ми 5-й октавы (E5)
  if (!incomingBuffer) incomingBuffer = generateBeep(659.25, 0.4, 0.15, 'incoming')
  playBuffer(incomingBuffer)
}

export function playOutgoing() {
  // Легкий клик Ля 4-й октавы (A4)
  if (!outgoingBuffer) outgoingBuffer = generateBeep(440, 0.15, 0.1, 'outgoing')
  playBuffer(outgoingBuffer)
}

export function playCallRing() {
  const buf = generateBeep(440, 0.5, 0.4, 'ring')
  playBuffer(buf)
}
