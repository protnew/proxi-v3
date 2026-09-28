/**
 * WebRTC tunnel — creates encrypted P2P Data Channel between peers
 * Uses Nostr signaling for ICE/SDP exchange
 */
import { NostrSignaling, type VPNSignal } from './nostr-signaling'

export type TunnelState = 'disconnected' | 'connecting' | 'connected'

export class WebRTCTunnel {
  private pc: RTCPeerConnection | null = null
  private dc: RTCDataChannel | null = null
  private signaling: NostrSignaling
  private targetPubkey: string
  private _state: TunnelState = 'disconnected'
  private onStateChange: ((state: TunnelState) => void) | null = null
  private onMessage: ((data: string) => void) | null = null

  // STUN servers for NAT traversal
  private static readonly ICE_SERVERS: RTCConfiguration = {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
      { urls: 'stun:stun.cloudflare.com:3478' },
    ],
  }

  constructor(signaling: NostrSignaling, targetPubkey: string) {
    this.signaling = signaling
    this.targetPubkey = targetPubkey
  }

  get state(): TunnelState {
    return this._state
  }

  private setState(s: TunnelState) {
    this._state = s
    this.onStateChange?.(s)
  }

  /**
   * Set callbacks
   */
  on(event: 'stateChange' | 'message', cb: (arg: any) => void) {
    if (event === 'stateChange') this.onStateChange = cb
    if (event === 'message') this.onMessage = cb
  }

  /**
   * Initiate connection to peer (caller side)
   */
  async connect() {
    this.setState('connecting')
    this.pc = new RTCPeerConnection(WebRTCTunnel.ICE_SERVERS)

    // Create data channel
    this.dc = this.pc.createDataChannel('vpn-tunnel', {
      ordered: false,
      maxRetransmits: 0,  // unreliable for speed (like UDP)
    })
    this.setupDataChannel(this.dc)

    // ICE candidates → send via Nostr
    this.pc.onicecandidate = (e) => {
      if (e.candidate) {
        this.signaling.sendIceCandidate(this.targetPubkey, e.candidate.toJSON())
      }
    }

    // Create offer
    const offer = await this.pc.createOffer()
    await this.pc.setLocalDescription(offer)
    await this.signaling.sendOffer(this.targetPubkey, offer.sdp!)
  }

  /**
   * Handle incoming signal from Nostr
   */
  async handleSignal(signal: VPNSignal) {
    switch (signal.type) {
      case 'offer':
        await this.handleOffer(signal.sdp!)
        break
      case 'answer':
        await this.handleAnswer(signal.sdp!)
        break
      case 'ice-candidate':
        await this.handleIce(signal.ice!)
        break
      case 'request':
        // Peer wants to connect — nothing to do here, they'll send offer
        break
    }
  }

  private async handleOffer(sdp: string) {
    this.setState('connecting')
    this.pc = new RTCPeerConnection(WebRTCTunnel.ICE_SERVERS)

    this.pc.ondatachannel = (e) => {
      this.dc = e.channel
      this.setupDataChannel(this.dc)
    }

    this.pc.onicecandidate = (e) => {
      if (e.candidate) {
        this.signaling.sendIceCandidate(this.targetPubkey, e.candidate.toJSON())
      }
    }

    await this.pc.setRemoteDescription({ type: 'offer', sdp })
    const answer = await this.pc.createAnswer()
    await this.pc.setLocalDescription(answer)
    await this.signaling.sendAnswer(this.targetPubkey, answer.sdp!)
  }

  private async handleAnswer(sdp: string) {
    if (!this.pc) return
    await this.pc.setRemoteDescription({ type: 'answer', sdp })
  }

  private async handleIce(candidate: RTCIceCandidateInit) {
    if (!this.pc) return
    await this.pc.addIceCandidate(new RTCIceCandidate(candidate))
  }

  private setupDataChannel(dc: RTCDataChannel) {
    dc.onopen = () => {
      this.setState('connected')
      console.log('[Tunnel] Data channel open!')
    }
    dc.onclose = () => {
      this.setState('disconnected')
      console.log('[Tunnel] Data channel closed')
    }
    dc.onmessage = (e) => {
      this.onMessage?.(e.data as string)
    }
  }

  /**
   * Send data through the tunnel
   */
  send(data: string) {
    if (this.dc?.readyState === 'open') {
      this.dc.send(data)
    }
  }

  /**
   * Send HTTP request through tunnel (for proxy mode)
   */
  sendHttpRequest(url: string, method: string = 'GET'): Promise<string> {
    return new Promise((resolve, reject) => {
      const id = crypto.randomUUID()
      const handler = (msg: string) => {
        try {
          const resp = JSON.parse(msg)
          if (resp.id === id && resp.type === 'http-response') {
            this.onMessage = null
            resolve(resp.body)
          }
        } catch {}
      }
      this.onMessage = handler
      this.send(JSON.stringify({ type: 'http-request', id, url, method }))

      // Timeout after 30s
      setTimeout(() => {
        this.onMessage = null
        reject(new Error('Request timeout'))
      }, 30000)
    })
  }

  /**
   * Disconnect
   */
  close() {
    this.dc?.close()
    this.pc?.close()
    this.dc = null
    this.pc = null
    this.setState('disconnected')
  }
}
