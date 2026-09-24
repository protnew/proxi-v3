/**
 * OFF-001 — NIP-59 Gift Wrap for offline VPN invites / requests.
 *
 * Spec: https://github.com/nostr-protocol/nips/blob/master/59.md
 * Layers: rumor (kind 14-ish / custom 30090 payload) → seal (kind 13) → wrap (kind 1059)
 *
 * Goal: friend can be offline; encrypted invite sits on Nostr relays until pickup.
 * Relays see only random ephemeral wrap pubkey — not Alice/Bob identities.
 */

import { wrapEvent, unwrapEvent } from 'nostr-tools/nip59'
import type { Event as NostrEvent, UnsignedEvent } from 'nostr-tools'

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.startsWith('0x') ? hex.slice(2) : hex
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
  return out
}

export type VpnInvitePayload = {
  type: 'vpn_invite' | 'vpn_request'
  from: string
  to: string
  onion?: string
  wtAddr?: string
  wtCertHash?: string
  token?: string
  exp?: number
  sdp?: string
  ice?: string
  note?: string
  ts: number
  v: 1
}

export type GiftWrapResult = {
  wrap: NostrEvent
  rumorId: string
}

/**
 * Create NIP-59 gift-wrapped VPN invite for offline delivery via Nostr relays.
 */
export function wrapVpnInvite(
  payload: VpnInvitePayload,
  senderPrivateKeyHex: string,
  recipientPublicKeyHex: string,
): GiftWrapResult {
  const sk = hexToBytes(senderPrivateKeyHex)
  const pk = recipientPublicKeyHex.length === 66 && (recipientPublicKeyHex.startsWith('02') || recipientPublicKeyHex.startsWith('03'))
    ? recipientPublicKeyHex.slice(2)
    : recipientPublicKeyHex

  const event: Partial<UnsignedEvent> = {
    kind: 14,
    content: JSON.stringify(payload),
    tags: [
      ['p', pk],
      ['t', payload.type],
      ['app', 'indestructible-vpn'],
    ],
    created_at: Math.floor(Date.now() / 1000),
  }

  const wrap = wrapEvent(event, sk, pk)
  return { wrap, rumorId: wrap.id }
}

/**
 * Unwrap gift-wrapped event with recipient private key.
 * Returns VpnInvitePayload or null if not our VPN app payload.
 */
export function unwrapVpnInvite(
  wrap: NostrEvent,
  recipientPrivateKeyHex: string,
): VpnInvitePayload | null {
  try {
    const sk = hexToBytes(recipientPrivateKeyHex)
    const rumor = unwrapEvent(wrap, sk)
    const data = JSON.parse(rumor.content) as VpnInvitePayload
    if (!data || (data.type !== 'vpn_invite' && data.type !== 'vpn_request')) return null
    if (data.v !== 1) return null
    return data
  } catch {
    return null
  }
}

/**
 * Publish wrap to connected Nostr relays (best-effort).
 * Expects window.__nostrWS or array of WebSockets on window.__nostrRelays.
 */
export function publishGiftWrap(wrap: NostrEvent): number {
  let sent = 0
  const msg = JSON.stringify(['EVENT', wrap])
  try {
    const w = (window as any).__nostrWS
    if (w && w.readyState === 1) {
      w.send(msg)
      sent++
    }
  } catch { /* ignore */ }
  try {
    const relays: WebSocket[] = (window as any).__nostrRelaySockets || []
    for (const ws of relays) {
      if (ws && ws.readyState === 1) {
        ws.send(msg)
        sent++
      }
    }
  } catch { /* ignore */ }
  // Also try NostrChat global if present
  try {
    const nc = (window as any).__nostrChat
    if (nc?.publishRaw) {
      nc.publishRaw(wrap)
      sent++
    } else if (nc?.pool?.publish) {
      // generic
      sent++
    }
  } catch { /* ignore */ }
  console.log('[gift-wrap] published to', sent, 'sockets, id=', wrap.id?.slice(0, 12))
  return sent
}
