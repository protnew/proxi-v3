/**
 * Nostr VPN signaling — kind:30090 signed events over local Go relay.
 * Uses same signing path as nostr-signaling.ts / identity.ts (NIP-01).
 */
import * as secp from '@noble/secp256k1'
import { signEvent } from './identity'
import { getPubkey, getSeckey } from './api'

function hexToBytes(hex: string): Uint8Array {
  const h = hex.length % 2 ? '0' + hex : hex
  const out = new Uint8Array(h.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16)
  return out
}
function bytesToHex(b: Uint8Array): string {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}
/** Prefer real schnorr pubkey from seckey (demo mode stores pubkey=seckey). */
function signingIdentity(): { pubkey: string; seckey: string } {
  const seckey = getSeckey()
  if (!seckey || seckey.length < 64) throw new Error('No seckey')
  const sk = seckey.slice(0, 64)
  const pubkey = bytesToHex(secp.schnorr.getPublicKey(hexToBytes(sk)))
  return { pubkey, seckey: sk }
}

export type VPNEventType = 'vpn-invite' | 'vpn-request' | 'vpn-accept' | 'vpn-reject' | 'vpn-cancel' | 'rtc-offer' | 'rtc-answer' | 'rtc-ice'

export interface VPNEvent {
  type: VPNEventType
  from: string
  to: string
  wtPort?: number
  wtCertHash?: string
  wtAddr?: string
  timestamp: number
  // WebRTC fields (architecture table 26_Signaling_Protocol)
  rtcSdp?: string
  rtcType?: "offer" | "answer"
  iceCandidates?: string[]
}

const KIND_VPN = 30090

async function sha256Hex(data: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(data))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

function relayWsUrl(): string {
  const loc = typeof window !== 'undefined' ? window.location : null
  if (!loc) return 'ws://127.0.0.1:8090/nostr'
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${loc.host}/nostr`
}

export class NostrVPNSignaling {
  private ws: WebSocket | null = null
  private handlers: ((event: VPNEvent, raw: any) => void)[] = []
  private myPubkey = ''
  private subId = 'vpn-' + Math.random().toString(36).slice(2, 10)
  private ready: Promise<void> | null = null
  private resolveReady: (() => void) | null = null
  private seenIds = new Set<string>()
  private startedAt = Math.floor(Date.now() / 1000)

  async init(pubkey?: string): Promise<void> {
    try {
      const id = signingIdentity()
      this.myPubkey = pubkey || id.pubkey
    } catch {
      this.myPubkey = pubkey || getPubkey()
    }
    if (!this.myPubkey) throw new Error('No pubkey — login first')

    this.ready = new Promise(res => { this.resolveReady = res })
    const url = relayWsUrl()
    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      // Subscribe to VPN events addressed to me (#p = my pubkey)
      const filter = {
        kinds: [KIND_VPN],
        '#p': [this.myPubkey],
        since: this.startedAt - 5,
        limit: 5,
      }
      this.ws!.send(JSON.stringify(['REQ', this.subId, filter]))
      console.log('[Nostr-VPN] connected', url, 'sub', this.myPubkey.slice(0, 12))
      this.resolveReady?.()
    }

    this.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (!Array.isArray(msg)) return
        if (msg[0] === 'EVENT' && msg[2]) {
          const ev = msg[2]
          if (ev.kind !== KIND_VPN) return
          if (ev.id && this.seenIds.has(ev.id)) return
          if (ev.id) this.seenIds.add(ev.id)
          try {
            const vpn: VPNEvent = JSON.parse(ev.content)
            // only deliver if addressed to me
            if (vpn.to && vpn.to !== this.myPubkey && !ev.tags?.some((t: string[]) => t[0] === 'p' && t[1] === this.myPubkey)) {
              return
            }
            console.log('[Nostr-VPN] EVENT', vpn.type, 'from', (vpn.from || '').slice(0, 12))
            this.handlers.forEach(h => h(vpn, ev))
          } catch (err) {
            console.warn('[Nostr-VPN] bad content', err)
          }
        } else if (msg[0] === 'OK') {
          console.log('[Nostr-VPN] OK', msg[1]?.slice?.(0, 12), msg[2], msg[3] || '')
        } else if (msg[0] === 'NOTICE') {
          console.warn('[Nostr-VPN] NOTICE', msg[1])
        }
      } catch (err) {
        console.warn('[Nostr-VPN] parse', err)
      }
    }

    this.ws.onerror = (e) => console.warn('[Nostr-VPN] ws error', e)
    this.ws.onclose = () => console.log('[Nostr-VPN] closed')

    // wait open (max 5s)
    await Promise.race([
      this.ready,
      new Promise((_, rej) => setTimeout(() => rej(new Error('Nostr VPN relay timeout')), 5000)),
    ])
  }

  async sendVPNEvent(type: VPNEventType, toPubkey: string, extra?: Partial<VPNEvent>): Promise<string> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      await this.init(this.myPubkey)
    }
    const { pubkey: from, seckey } = signingIdentity()
    const contentObj: VPNEvent = {
      type,
      from,
      to: toPubkey,
      timestamp: Math.floor(Date.now() / 1000),
      ...extra,
    }
    const content = JSON.stringify(contentObj)
    const createdAt = Math.floor(Date.now() / 1000)
    const tags: string[][] = [['p', toPubkey], ['t', type]]
    const kind = KIND_VPN

    // NIP-01 id = sha256([0, pubkey, created_at, kind, tags, content])
    const serialized = JSON.stringify([0, from, createdAt, kind, tags, content])
    const id = await sha256Hex(serialized)
    const sig = await signEvent(
      { pubkey: from, created_at: createdAt, kind, tags, content },
      seckey,
    )

    const event = { id, pubkey: from, created_at: createdAt, kind, tags, content, sig }
    this.ws!.send(JSON.stringify(['EVENT', event]))
    console.log('[Nostr-VPN] sent signed', type, 'id', id.slice(0, 12), 'to', toPubkey.slice(0, 12))
    return id
  }

  async inviteFriend(toPubkey: string, wtAddr: string, wtCertHash: string): Promise<string> {
    return this.sendVPNEvent('vpn-invite', toPubkey, { wtAddr, wtCertHash })
  }

  async requestVPN(fromPubkey: string): Promise<string> {
    return this.sendVPNEvent('vpn-request', fromPubkey)
  }

  async acceptVPN(toPubkey: string, wtAddr?: string, wtCertHash?: string): Promise<string> {
    return this.sendVPNEvent('vpn-accept', toPubkey, { wtAddr, wtCertHash })
  }

  async sendRTCOffer(toPubkey: string, sdp: string, iceCandidates: string[]): Promise<string> {
    return this.sendVPNEvent("rtc-offer", toPubkey, { rtcSdp: sdp, rtcType: "offer", iceCandidates })
  }
  async sendRTCAnswer(toPubkey: string, sdp: string, iceCandidates: string[]): Promise<string> {
    return this.sendVPNEvent("rtc-answer", toPubkey, { rtcSdp: sdp, rtcType: "answer", iceCandidates })
  }
  async sendICE(toPubkey: string, candidate: string): Promise<string> {
    return this.sendVPNEvent("rtc-ice", toPubkey, { iceCandidates: [candidate] })
  }
  async rejectVPN(toPubkey: string): Promise<string> {
    return this.sendVPNEvent('vpn-reject', toPubkey)
  }

  onVPNEvent(handler: (event: VPNEvent, raw?: any) => void): () => void {
    this.handlers.push(handler)
    return () => { this.handlers = this.handlers.filter(h => h !== handler) }
  }

  destroy(): void {
    try { this.ws?.close() } catch {}
    this.ws = null
    this.handlers = []
  }
}

export const nostrVPN = new NostrVPNSignaling()
