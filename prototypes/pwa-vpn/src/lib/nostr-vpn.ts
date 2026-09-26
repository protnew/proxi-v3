import { wsAuthProtocols } from './ws-auth'
/**
 * Nostr VPN signaling — kind:30090 signed events over local Go relay.
 * Uses same signing path as nostr-signaling.ts / identity.ts (NIP-01).
 */
import * as secp from '@noble/secp256k1'
import { signEvent } from './identity'
import { getPubkey, getSeckey } from './api'
import { wrapVpnInvite, unwrapVpnInvite, publishGiftWrap, type VpnInvitePayload } from './nip59-giftwrap'

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

/** @deprecated R26b: new invites are kind 1059. 30090 is replay-only. */
const KIND_VPN = 30090

async function sha256Hex(data: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(data))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

function relayWsUrl(): string {
  const loc = typeof window !== 'undefined' ? window.location : null
  if (!loc) return 'ws://127.0.0.1:8090/nostr'
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  // P9: JWT rides in Sec-WebSocket-Protocol, not the URL.
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
    const tok = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_token') : null
    this.ws = new WebSocket(url, wsAuthProtocols(tok))

    this.ws.onopen = () => {
      // Subscribe to VPN events addressed to me (#p = my pubkey)
      const filter = {
        kinds: [KIND_VPN],
        '#p': [this.myPubkey],
        since: this.startedAt - 5,
        limit: 5,
      }
      this.ws!.send(JSON.stringify(['REQ', this.subId, filter]))
      // OFF-001: also subscribe to NIP-59 gift wraps (kind 1059) addressed to me
      const wrapSub = this.subId + '-gw'
      this.ws!.send(JSON.stringify(['REQ', wrapSub, {
        kinds: [1059],
        '#p': [this.myPubkey],
        since: this.startedAt - 3600,
        limit: 20,
      }]))
      console.log('[Nostr-VPN] connected', url, 'sub', this.myPubkey.slice(0, 12))
      this.resolveReady?.()
    }

    this.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (!Array.isArray(msg)) return
        if (msg[0] === 'EVENT' && msg[2]) {
          const ev = msg[2]
          if (ev.kind === 1059) {
            // Gift wrap offline invite
            try {
              const sk = getSeckey()
              if (!sk) return
              const payload = unwrapVpnInvite(ev, sk.slice(0, 64))
              if (!payload) return
              const vpn: VPNEvent = {
                type: payload.type === 'vpn_request' ? 'vpn-request' : 'vpn-invite',
                from: payload.from,
                to: payload.to,
                timestamp: Math.floor((payload.ts || Date.now()) / 1000),
                rtcSdp: payload.sdp,
                iceCandidates: payload.ice ? JSON.parse(payload.ice) : undefined,
              }
              console.log('[Nostr-VPN] GIFT-WRAP', vpn.type, 'from', vpn.from.slice(0, 12))
              this.handlers.forEach(h => h(vpn, ev))
            } catch (err) {
              console.warn('[Nostr-VPN] gift unwrap failed', err)
            }
            return
          }
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

  async inviteFriend(toPubkey: string, wtAddr: string, wtCertHash: string, onion?: string): Promise<string> {
    if (!toPubkey) throw new Error('invite needs recipient')
    if (!wtAddr && !onion) throw new Error('invite endpoint empty')
    const { seckey, pubkey } = signingIdentity()
    const now = Math.floor(Date.now() / 1000)
    const payload: VpnInvitePayload = {
      type: 'vpn_invite',
      from: pubkey,
      to: toPubkey,
      onion: onion || undefined,
      wtAddr: wtAddr || undefined,
      wtCertHash: wtCertHash || undefined,
      token: crypto.randomUUID().replace(/-/g, ''),
      exp: now + 15 * 60,
      ts: now,
      v: 1,
    }
    const url = encodeInviteURL(payload)
    const { wrap } = wrapVpnInvite(payload, seckey, toPubkey)
    if (wrap.kind !== 1059) throw new Error('invite must be gift-wrap 1059')
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(['EVENT', wrap]))
    }
    publishGiftWrap(wrap)
    return url
  }

  async requestVPN(fromPubkey: string): Promise<string> {
    const id = await this.sendVPNEvent('vpn-request', fromPubkey)
    try {
      const { seckey, pubkey } = signingIdentity()
      const payload: VpnInvitePayload = {
        type: 'vpn_request',
        from: pubkey,
        to: fromPubkey,
        ts: Date.now(),
        v: 1,
      }
      const { wrap } = wrapVpnInvite(payload, seckey, fromPubkey)
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(['EVENT', wrap]))
      }
      publishGiftWrap(wrap)
    } catch (e) {
      console.warn('[Nostr-VPN] gift-wrap request failed', e)
    }
    return id
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


export function encodeInviteURL(payload: VpnInvitePayload): string {
  const raw = JSON.stringify(payload)
  const b64 = btoa(unescape(encodeURIComponent(raw))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
  return 'proxi+vpn://v1?' + b64
}

export function decodeInviteURL(url: string): VpnInvitePayload {
  const q = url.split('?')[1] || ''
  const pad = q.replace(/-/g, '+').replace(/_/g, '/')
  const raw = decodeURIComponent(escape(atob(pad + '==='.slice((pad.length + 3) % 4))))
  const payload = JSON.parse(raw) as VpnInvitePayload
  if (!payload.to || !payload.token) throw new Error('invite missing to or token')
  if (!payload.wtAddr && !payload.onion) throw new Error('invite endpoint empty')
  if (payload.exp > payload.ts + 24 * 3600) throw new Error('invite exp too far')
  return payload
}

export const nostrVPN = new NostrVPNSignaling()
