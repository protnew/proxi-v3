/**
 * WebRTC Peer Connection Manager
 * Manages P2P connections between messenger users
 * Used for: file transfer, voice messages, calls
 */
import { getPubkey, getSeckey, sendCallSignal } from './api'
import { FileReceiver } from './file-transfer'
import type { FileManifest } from './file-transfer'

const ICE_SERVERS: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
  ],
}

// Active peer connections: pubkey → RTCPeerConnection + DataChannel
const peers = new Map<string, {
  pc: RTCPeerConnection
  dc: RTCDataChannel | null
  iceBuffer: RTCIceCandidateInit[]
  remoteDescSet: boolean
  fileReceiver: FileReceiver
}>()

let onFileComplete: ((blob: Blob, manifest: FileManifest) => void) | null = null

export function setOnFileComplete(cb: (blob: Blob, manifest: FileManifest) => void) {
  onFileComplete = cb
}

/**
 * Get or create peer connection
 */
function getPeer(pubkey: string): RTCDataChannel | null {
  const existing = peers.get(pubkey)
  if (existing?.dc?.readyState === 'open') return existing.dc

  // Create new connection
  const pc = new RTCPeerConnection(ICE_SERVERS)
  const peer: typeof peers extends Map<string, infer V> ? V : never = {
    pc,
    dc: null,
    iceBuffer: [],
    remoteDescSet: false,
    fileReceiver: new FileReceiver(),
  }
  peers.set(pubkey, peer)

  // Create data channel
  const dc = pc.createDataChannel('files', { ordered: true })
  peer.dc = dc
  setupDataChannel(dc, pubkey)

  // ICE candidates → send via Nostr
  pc.onicecandidate = (e) => {
    if (e.candidate) {
      sendCallSignal(pubkey, { type: 'ice', candidate: e.candidate.toJSON() })
    }
  }

  pc.ondatachannel = (e) => {
    peer.dc = e.channel
    setupDataChannel(e.channel, pubkey)
  }

  return dc
}

function setupDataChannel(dc: RTCDataChannel, peerPubkey: string) {
  dc.binaryType = 'arraybuffer'
  dc.onopen = () => console.log(`[P2P] Connected to ${peerPubkey.slice(0, 8)}`)
  dc.onclose = () => console.log(`[P2P] Disconnected from ${peerPubkey.slice(0, 8)}`)

  let expectedChunk: { manifestId: string; index: number } | null = null

  dc.onmessage = (e) => {
    const peer = peers.get(peerPubkey)
    if (!peer) return

    if (typeof e.data === 'string') {
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'file-manifest') {
          peer.fileReceiver.handleMessage(e.data)
          peer.fileReceiver.register(msg.manifest.id, {
            onComplete: (blob, manifest) => {
              onFileComplete?.(blob, manifest)
            }
          })
        }
        if (msg.type === 'file-chunk') {
          expectedChunk = { manifestId: msg.manifestId, index: msg.index }
        }
        if (msg.type === 'file-complete') {
          peer.fileReceiver.complete(msg.manifestId)
        }
        // Voice/call signaling can also go here
      } catch {}
    } else if (e.data instanceof ArrayBuffer && expectedChunk) {
      const peer2 = peers.get(peerPubkey)
      peer2?.fileReceiver.handleChunk(expectedChunk.manifestId, expectedChunk.index, e.data)
      expectedChunk = null
    }
  }
}

/**
 * Initiate P2P connection to a peer (creates offer)
 */
export async function connectToPeer(pubkey: string): Promise<RTCDataChannel | null> {
  const peer = peers.get(pubkey)
  if (peer?.dc?.readyState === 'open') return peer.dc

  // Clean up old
  peer?.pc.close()

  const pc = new RTCPeerConnection(ICE_SERVERS)
  const newPeer = {
    pc, dc: null as RTCDataChannel | null,
    iceBuffer: [], remoteDescSet: false,
    fileReceiver: new FileReceiver(),
  }
  peers.set(pubkey, newPeer)

  pc.onicecandidate = (e) => {
    if (e.candidate) sendCallSignal(pubkey, { type: 'ice', candidate: e.candidate.toJSON() })
  }

  pc.ondatachannel = (e) => {
    newPeer.dc = e.channel
    setupDataChannel(e.channel, pubkey)
  }

  // Create offer
  const dc = pc.createDataChannel('files', { ordered: true })
  newPeer.dc = dc
  setupDataChannel(dc, pubkey)

  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)

  // Wait for ICE gathering
  await new Promise<void>(resolve => {
    if (pc.iceGatheringState === 'complete') return resolve()
    pc.onicegatheringstatechange = () => { if (pc.iceGatheringState === 'complete') resolve() }
    setTimeout(resolve, 3000)
  })

  sendCallSignal(pubkey, { type: 'offer', sdp: pc.localDescription!.sdp })
  return dc
}

/**
 * Handle incoming signaling (from Nostr)
 */
export async function handleSignal(fromPubkey: string, signal: any) {
  if (signal.type === 'offer') {
    await handleOffer(fromPubkey, signal.sdp)
  } else if (signal.type === 'answer') {
    await handleAnswer(fromPubkey, signal.sdp)
  } else if (signal.type === 'ice') {
    await handleIce(fromPubkey, signal.candidate)
  }
}

async function handleOffer(fromPubkey: string, sdp: string) {
  let peer = peers.get(fromPubkey)
  if (peer) peer.pc.close()

  const pc = new RTCPeerConnection(ICE_SERVERS)
  const newPeer = {
    pc, dc: null as RTCDataChannel | null,
    iceBuffer: [], remoteDescSet: false,
    fileReceiver: new FileReceiver(),
  }
  peers.set(fromPubkey, newPeer)

  pc.onicecandidate = (e) => {
    if (e.candidate) sendCallSignal(fromPubkey, { type: 'ice', candidate: e.candidate.toJSON() })
  }
  pc.ondatachannel = (e) => {
    newPeer.dc = e.channel
    setupDataChannel(e.channel, fromPubkey)
  }

  await pc.setRemoteDescription({ type: 'offer', sdp })
  newPeer.remoteDescSet = true

  const answer = await pc.createAnswer()
  await pc.setLocalDescription(answer)

  await new Promise<void>(resolve => {
    if (pc.iceGatheringState === 'complete') return resolve()
    pc.onicegatheringstatechange = () => { if (pc.iceGatheringState === 'complete') resolve() }
    setTimeout(resolve, 3000)
  })

  sendCallSignal(fromPubkey, { type: 'answer', sdp: pc.localDescription!.sdp })

  // Flush buffered ICE
  for (const c of newPeer.iceBuffer) {
    try { await pc.addIceCandidate(new RTCIceCandidate(c)) } catch {}
  }
  newPeer.iceBuffer = []
}

async function handleAnswer(fromPubkey: string, sdp: string) {
  const peer = peers.get(fromPubkey)
  if (!peer) return
  await peer.pc.setRemoteDescription({ type: 'answer', sdp })
  peer.remoteDescSet = true
  for (const c of peer.iceBuffer) {
    try { await peer.pc.addIceCandidate(new RTCIceCandidate(c)) } catch {}
  }
  peer.iceBuffer = []
}

async function handleIce(fromPubkey: string, candidate: RTCIceCandidateInit) {
  const peer = peers.get(fromPubkey)
  if (!peer) return
  if (peer.remoteDescSet) {
    try { await peer.pc.addIceCandidate(new RTCIceCandidate(candidate)) } catch {}
  } else {
    peer.iceBuffer.push(candidate)
  }
}

/**
 * Send a file to a peer
 */
export async function sendFile(
  pubkey: string,
  file: File,
  onProgress?: (sent: number, total: number) => void
): Promise<void> {
  const dc = await connectToPeer(pubkey)
  if (!dc) throw new Error('Failed to create P2P connection')

  const { createManifest, sendChunks } = await import('./file-transfer')
  const { manifest, chunks } = await createManifest(file, getPubkey())
  await sendChunks(dc, manifest, chunks, onProgress)
}
