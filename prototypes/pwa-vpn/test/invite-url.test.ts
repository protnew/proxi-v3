import { describe, expect, it } from 'vitest'
import { decodeInviteURL, encodeInviteURL } from '../src/lib/nostr-vpn'

describe('invite url', () => {
  it('round-trips endpoint and keeps exp in seconds', () => {
    const now = 1_700_000_000
    const url = encodeInviteURL({
      type: 'vpn_invite', from: 'aa', to: 'bb', wtAddr: '203.0.113.9:443',
      token: 'tok', exp: now + 900, ts: now, v: 1,
    })
    expect(url.startsWith('proxi+vpn://v1?')).toBe(true)
    expect(url).not.toContain('token=')
    const got = decodeInviteURL(url)
    expect(got.wtAddr).toBe('203.0.113.9:443')
    expect(got.exp).toBe(now + 900)
    expect(String(got.exp).length).toBe(10)
  })
  it('rejects an invite with no endpoint', () => {
    const url = encodeInviteURL({ type: 'vpn_invite', from: 'aa', to: 'bb', token: 'tok', exp: 10, ts: 1, v: 1 })
    expect(() => decodeInviteURL(url)).toThrow(/endpoint empty/)
  })
})
