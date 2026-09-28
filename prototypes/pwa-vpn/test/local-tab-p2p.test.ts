/**
 * VPN-101: LocalTabP2P unit — BroadcastChannel signaling + mock WebRTC
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

class BCPoly {
  static buses = new Map<string, Set<BCPoly>>()
  name: string
  onmessage: ((ev: MessageEvent) => void) | null = null
  constructor(name: string) {
    this.name = name
    if (!BCPoly.buses.has(name)) BCPoly.buses.set(name, new Set())
    BCPoly.buses.get(name)!.add(this)
  }
  postMessage(data: unknown) {
    for (const peer of BCPoly.buses.get(this.name) || []) {
      if (peer === this) continue
      // sync deliver — real BC is async but sync is ok for unit
      peer.onmessage?.({ data } as MessageEvent)
    }
  }
  close() {
    BCPoly.buses.get(this.name)?.delete(this)
  }
}

class FakeDC {
  readyState: RTCDataChannelState = 'connecting'
  binaryType = 'arraybuffer'
  onopen: ((ev: Event) => void) | null = null
  onmessage: ((ev: MessageEvent) => void) | null = null
  onclose: ((ev: Event) => void) | null = null
  onerror: ((ev: Event) => void) | null = null
  peer: FakeDC | null = null
  link(p: FakeDC) { this.peer = p; p.peer = this }
  open() {
    this.readyState = 'open'
    this.onopen?.(new Event('open'))
  }
  send(data: ArrayBuffer | Uint8Array) {
    const u8 = data instanceof Uint8Array ? data : new Uint8Array(data)
    const copy = u8.slice()
    this.peer?.onmessage?.({ data: copy.buffer } as MessageEvent)
  }
  close() { this.readyState = 'closed'; this.onclose?.(new Event('close')) }
}

const pair = { callerDc: null as FakeDC | null, calleePc: null as FakePC | null }

class FakePC {
  iceGatheringState: RTCIceGatheringState = 'new'
  localDescription: RTCSessionDescriptionInit | null = null
  remoteDescription: RTCSessionDescriptionInit | null = null
  onicecandidate: ((ev: any) => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  ondatachannel: ((ev: any) => void) | null = null
  private isCaller = false

  createDataChannel() {
    this.isCaller = true
    const dc = new FakeDC()
    pair.callerDc = dc
    return dc as any
  }
  async createOffer() {
    return { type: 'offer', sdp: 'offer-sdp' }
  }
  async createAnswer() {
    return { type: 'answer', sdp: 'answer-sdp' }
  }
  async setLocalDescription(desc: RTCSessionDescriptionInit) {
    this.localDescription = desc
    this.iceGatheringState = 'complete'
    this.onicecandidate?.({ candidate: { candidate: 'fake', sdpMid: '0', sdpMLineIndex: 0 } })
    this.onicecandidate?.({ candidate: null })
    this.onicegatheringstatechange?.()
    this.finishPair()
  }
  async setRemoteDescription(desc: RTCSessionDescriptionInit) {
    this.remoteDescription = desc
    if (desc.type === 'offer') {
      pair.calleePc = this
      const dc = new FakeDC()
      this.ondatachannel?.({ channel: dc })
      this.finishPair()
    } else {
      this.finishPair()
    }
  }
  async addIceCandidate() {}
  close() {}
  private finishPair() {
    if (!pair.callerDc || !pair.calleePc) return
    // find callee dc via last ondatachannel — stored on caller link only
    // We need callee channel reference: re-create link through callerDc.peer
    if (!pair.callerDc.peer) {
      // callee channel was passed to ondatachannel; recover from weak store
      const calleeDc = (pair.calleePc as any)._dc as FakeDC | undefined
      // store on setRemoteDescription
    }
  }
}

// Better FakePC with explicit channel store
class FakePC2 {
  iceGatheringState: RTCIceGatheringState = 'new'
  onicecandidate: ((ev: any) => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  ondatachannel: ((ev: any) => void) | null = null
  dc: FakeDC | null = null
  static callerPc: FakePC2 | null = null
  static calleePc: FakePC2 | null = null

  createDataChannel() {
    this.dc = new FakeDC()
    FakePC2.callerPc = this
    return this.dc as any
  }
  async createOffer() { return { type: 'offer', sdp: 'o' } }
  async createAnswer() { FakePC2.calleePc = this; return { type: 'answer', sdp: 'a' } }
  async setLocalDescription() {
    this.iceGatheringState = 'complete'
    this.onicecandidate?.({ candidate: { candidate: 'c', sdpMid: '0', sdpMLineIndex: 0 } })
    this.onicecandidate?.({ candidate: null })
    this.onicegatheringstatechange?.()
    FakePC2.tryOpen()
  }
  async setRemoteDescription(desc: RTCSessionDescriptionInit) {
    if (desc.type === 'offer') {
      FakePC2.calleePc = this
      this.dc = new FakeDC()
      this.ondatachannel?.({ channel: this.dc })
    }
    FakePC2.tryOpen()
  }
  async addIceCandidate() {}
  close() { this.dc?.close() }
  static tryOpen() {
    const a = FakePC2.callerPc?.dc
    const b = FakePC2.calleePc?.dc
    if (a && b && a.readyState !== 'open') {
      a.link(b)
      a.open()
      b.open()
    }
  }
}

describe('VPN-101 LocalTabP2P', () => {
  beforeEach(() => {
    BCPoly.buses.clear()
    FakePC2.callerPc = null
    FakePC2.calleePc = null
    ;(globalThis as any).BroadcastChannel = BCPoly
    ;(globalThis as any).RTCPeerConnection = FakePC2
  })
  afterEach(() => {
    vi.resetModules()
  })

  it('host+joiner reach connected via BroadcastChannel', async () => {
    vi.resetModules()
    const mod = await import('../src/lib/local-tab-p2p')
    const host = new mod.LocalTabP2P()
    const joiner = new mod.LocalTabP2P()

    await host.startAsHost()
    await joiner.startAsJoiner()

    const t0 = Date.now()
    while (Date.now() - t0 < 5000) {
      if (host.snapshot().phase === 'connected' && joiner.snapshot().phase === 'connected') break
      await new Promise(r => setTimeout(r, 50))
    }

    if (host.snapshot().phase !== 'connected' || joiner.snapshot().phase !== 'connected') {
      throw new Error(JSON.stringify({ host: host.snapshot(), joiner: joiner.snapshot() }))
    }
    expect(host.snapshot().role).toBe('host')
    expect(joiner.snapshot().role).toBe('joiner')
    host.stop()
    joiner.stop()
  })
})
