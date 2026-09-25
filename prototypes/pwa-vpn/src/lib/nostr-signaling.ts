import { wsAuthProtocols } from './ws-auth'
/**
 * Nostr client — minimal WebSocket relay connection for VPN signaling
 * No external nostr-tools dependency — pure WebSocket + JSON
 */
import { signEvent, type Identity } from './identity'

// VPN signaling event kinds
export const KIND_VPN_REQUEST = 30090
export const KIND_VPN_OFFER = 30091
export const KIND_VPN_ANSWER = 30092
export const KIND_VPN_ICE = 30093

// P4: VPN signaling only via local embedded relay — SDP/ICE must not
// reach public relays (leaks IP/topology). /nostr requires JWT (P9).
function defaultRelays(): string[] {
  const loc = typeof window !== 'undefined' ? window.location : null
  if (!loc) return ['ws://127.0.0.1:8090/nostr']
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return [`${proto}//${loc.host}/nostr`]
}

export interface VPNSignal {
  type: 'request' | 'offer' | 'answer' | 'ice-candidate'
  from: string
  to: string
  sdp?: string
  ice?: RTCIceCandidateInit
}

async function sha256Hex(data: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(data))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

export class NostrSignaling {
  private relays: string[]
  private identity: Identity
  private connections: WebSocket[] = []
  private onSignal: ((signal: VPNSignal) => void) | null = null
  private subscriptionId = 'vpn-' + Math.random().toString(36).slice(2, 10)

  constructor(identity: Identity, relays: string[] = defaultRelays()) {
    this.relays = relays
    this.identity = identity
  }

  async start(callback: (signal: VPNSignal) => void) {
    this.onSignal = callback

    const filter = JSON.stringify([{
      kinds: [KIND_VPN_REQUEST, KIND_VPN_OFFER, KIND_VPN_ANSWER, KIND_VPN_ICE],
      '#p': [this.identity.publicKey],
      limit: 0,  // only new events
    }])

    const subMsg = JSON.stringify(['REQ', this.subscriptionId, ...JSON.parse(filter)])

    for (const relayUrl of this.relays) {
      try {
        const tok = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_token') : null
        const ws = new WebSocket(relayUrl, wsAuthProtocols(tok))
        ws.onopen = () => {
          ws.send(subMsg)
          console.log(`[Nostr] Connected to ${relayUrl.split('?')[0]}`)
        }
        ws.onmessage = (e) => {
          this.handleMessage(e.data)
        }
        ws.onerror = (e) => {
          console.warn(`[Nostr] Error from ${relayUrl}:`, e)
        }
        this.connections.push(ws)
      } catch (e) {
        console.warn(`[Nostr] Failed to connect to ${relayUrl}:`, e)
      }
    }
  }

  stop() {
    for (const ws of this.connections) {
      try { ws.close() } catch {}
    }
    this.connections = []
  }

  async sendRequest(targetPubkey: string) {
    await this.publish(KIND_VPN_REQUEST, targetPubkey, JSON.stringify({ type: 'request' }))
  }

  async sendOffer(targetPubkey: string, sdp: string) {
    await this.publish(KIND_VPN_OFFER, targetPubkey, JSON.stringify({ type: 'offer', sdp }))
  }

  async sendAnswer(targetPubkey: string, sdp: string) {
    await this.publish(KIND_VPN_ANSWER, targetPubkey, JSON.stringify({ type: 'answer', sdp }))
  }

  async sendIceCandidate(targetPubkey: string, candidate: RTCIceCandidateInit) {
    await this.publish(KIND_VPN_ICE, targetPubkey, JSON.stringify({ type: 'ice-candidate', candidate }))
  }

  private async publish(kind: number, targetPubkey: string, content: string) {
    const createdAt = Math.floor(Date.now() / 1000)
    const tags = [['p', targetPubkey]]
    
    // Build event
    const serialized = JSON.stringify([0, this.identity.publicKey, createdAt, kind, tags, content])
    const id = await sha256Hex(serialized)
    const sig = await signEvent(
      { pubkey: this.identity.publicKey, created_at: createdAt, kind, tags, content },
      this.identity.privateKey
    )

    const event = { id, pubkey: this.identity.publicKey, created_at: createdAt, kind, tags, content, sig }
    const msg = JSON.stringify(['EVENT', event])

    for (const ws of this.connections) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(msg)
      }
    }
  }

  private handleMessage(data: string) {
    try {
      const msg = JSON.parse(data)
      // Nostr: ['EVENT', subId, event]
      if (msg[0] === 'EVENT' && msg[2]) {
        const event = msg[2]
        if (!this.onSignal) return
        
        const content = JSON.parse(event.content)
        this.onSignal({
          type: content.type,
          from: event.pubkey,
          to: this.identity.publicKey,
          sdp: content.sdp,
          ice: content.candidate,
        })
      }
    } catch (e) {
      // Not a VPN signal event, ignore
    }
  }
}
