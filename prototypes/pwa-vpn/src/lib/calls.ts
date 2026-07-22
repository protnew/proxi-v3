/**
 * WebRTC Audio/Video Calls
 * 1-на-1 calls with signaling over Nostr
 */

import { sendCallSignal } from './api'

export type CallState = 'idle' | 'ringing' | 'connecting' | 'connected' | 'ended'

let callState: CallState = 'idle'
let peerConnection: RTCPeerConnection | null = null
let localStream: MediaStream | null = null
let remoteStream: MediaStream | null = null
let onStateChange: ((state: CallState) => void) | null = null
let currentPeer: string = ''

const CALL_ICE: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
  ],
}

export function setOnCallStateChange(cb: (state: CallState) => void) {
  onStateChange = cb
}

export function getCallState() { return callState }
export function getLocalStream() { return localStream }
export function getRemoteStream() { return remoteStream }

function setState(s: CallState) {
  callState = s
  onStateChange?.(s)
}

/**
 * Start a call (caller side)
 */
export async function startCall(peerPubkey: string, audioOnly: boolean = true) {
  currentPeer = peerPubkey
  setState('connecting')
  
  try {
    localStream = await navigator.mediaDevices.getUserMedia({
      audio: true,
      video: !audioOnly,
    })
  } catch {
    setState('ended')
    return
  }

  peerConnection = new RTCPeerConnection(CALL_ICE)
  setupPeerConnection()

  // Add local tracks
  for (const track of localStream.getTracks()) {
    peerConnection.addTrack(track, localStream)
  }

  const offer = await peerConnection.createOffer()
  await peerConnection.setLocalDescription(offer)

  // Wait for ICE
  await new Promise<void>(resolve => {
    if (peerConnection!.iceGatheringState === 'complete') return resolve()
    peerConnection!.onicegatheringstatechange = () => {
      if (peerConnection!.iceGatheringState === 'complete') resolve()
    }
    setTimeout(resolve, 3000)
  })

  sendCallSignal(peerPubkey, {
    type: 'call-offer',
    sdp: peerConnection.localDescription!.sdp,
    audioOnly,
  })
}

/**
 * Accept incoming call
 */
export async function acceptCall(peerPubkey: string, sdp: string) {
  currentPeer = peerPubkey
  setState('connecting')

  try {
    localStream = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch {
    setState('ended')
    return
  }

  peerConnection = new RTCPeerConnection(CALL_ICE)
  setupPeerConnection()

  for (const track of localStream.getTracks()) {
    peerConnection.addTrack(track, localStream)
  }

  await peerConnection.setRemoteDescription({ type: 'offer', sdp })
  const answer = await peerConnection.createAnswer()
  await peerConnection.setLocalDescription(answer)

  await new Promise<void>(resolve => {
    if (peerConnection!.iceGatheringState === 'complete') return resolve()
    peerConnection!.onicegatheringstatechange = () => {
      if (peerConnection!.iceGatheringState === 'complete') resolve()
    }
    setTimeout(resolve, 3000)
  })

  sendCallSignal(peerPubkey, {
    type: 'call-answer',
    sdp: peerConnection.localDescription!.sdp,
  })
}

/**
 * Reject incoming call
 */
export function rejectCall(peerPubkey: string) {
  sendCallSignal(peerPubkey, { type: 'call-reject' })
  setState('idle')
}

/**
 * End current call
 */
export function endCall() {
  if (currentPeer) {
    sendCallSignal(currentPeer, { type: 'call-end' })
  }
  cleanup()
  setState('ended')
}

function setupPeerConnection() {
  if (!peerConnection) return

  peerConnection.onicecandidate = (e) => {
    if (e.candidate) {
      sendCallSignal(currentPeer, { type: 'call-ice', candidate: e.candidate.toJSON() })
    }
  }

  peerConnection.ontrack = (e) => {
    remoteStream = e.streams[0]
  }

  peerConnection.onconnectionstatechange = () => {
    if (peerConnection?.connectionState === 'connected') {
      setState('connected')
    } else if (peerConnection?.connectionState === 'disconnected' || peerConnection?.connectionState === 'failed') {
      cleanup()
      setState('ended')
    }
  }
}

/**
 * Handle call signaling from Nostr
 */
export async function handleCallSignal(fromPubkey: string, signal: any) {
  switch (signal.type) {
    case 'call-offer':
      // Incoming call
      currentPeer = fromPubkey
      setState('ringing')
      // Store SDP for accept
      ;(window as any).__pendingCallOffer = { from: fromPubkey, sdp: signal.sdp }
      break

    case 'call-answer':
      if (peerConnection && currentPeer === fromPubkey) {
        await peerConnection.setRemoteDescription({ type: 'answer', sdp: signal.sdp })
      }
      break

    case 'call-ice':
      if (peerConnection) {
        try {
          await peerConnection.addIceCandidate(new RTCIceCandidate(signal.candidate))
        } catch {}
      }
      break

    case 'call-reject':
    case 'call-end':
      cleanup()
      setState('ended')
      break
  }
}

function cleanup() {
  localStream?.getTracks().forEach(t => t.stop())
  peerConnection?.close()
  localStream = null
  remoteStream = null
  peerConnection = null
  currentPeer = ''
}
