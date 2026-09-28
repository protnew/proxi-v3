import { describe, it, expect } from 'vitest'
import { wrapVpnInvite, unwrapVpnInvite } from '../src/lib/nip59-giftwrap'
import * as secp from '@noble/secp256k1'

function bytesToHex(b: Uint8Array) {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}
function genPair() {
  const skBytes = crypto.getRandomValues(new Uint8Array(32))
  const pkBytes = secp.schnorr.getPublicKey(skBytes)
  return { sk: bytesToHex(skBytes), pk: bytesToHex(pkBytes) }
}

describe('NIP-59 Gift Wrap VPN invite (OFF-001)', () => {
  it('wrap + unwrap vpn_invite', () => {
    const a = genPair()
    const b = genPair()
    const { wrap } = wrapVpnInvite({
      type: 'vpn_invite',
      from: a.pk,
      to: b.pk,
      note: 'webrtc-ready',
      sdp: 'v=0 fake-sdp',
      ts: Date.now(),
      v: 1,
    }, a.sk, b.pk)

    expect(wrap.kind).toBe(1059)
    expect(wrap.pubkey).toBeTruthy()
    // wrap pubkey should NOT be Alice (ephemeral)
    expect(wrap.pubkey).not.toBe(a.pk)

    const payload = unwrapVpnInvite(wrap as any, b.sk)
    expect(payload).toBeTruthy()
    expect(payload!.type).toBe('vpn_invite')
    expect(payload!.from).toBe(a.pk)
    expect(payload!.sdp).toBe('v=0 fake-sdp')
  })

  it('wrong recipient cannot unwrap', () => {
    const a = genPair()
    const b = genPair()
    const c = genPair()
    const { wrap } = wrapVpnInvite({
      type: 'vpn_request',
      from: a.pk,
      to: b.pk,
      ts: Date.now(),
      v: 1,
    }, a.sk, b.pk)
    const payload = unwrapVpnInvite(wrap as any, c.sk)
    expect(payload).toBeNull()
  })
})
