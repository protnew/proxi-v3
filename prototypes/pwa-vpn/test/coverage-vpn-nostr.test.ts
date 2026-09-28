import { describe, it, expect, vi, beforeEach } from 'vitest'
import { hexToBytes, bytesToHex, normalizePeerId, authHeaders } from '../src/lib/vpn-utils.svelte'
import { NostrDataRelay } from '../src/lib/nostr-data-relay'
import { NostrVPNSignaling } from '../src/lib/nostr-vpn'

describe('vpn-utils', () => {
  it('hex roundtrip', () => {
    const b = hexToBytes('0a0b0c')
    expect([...b]).toEqual([10, 11, 12])
    expect(bytesToHex(b)).toBe('0a0b0c')
  })
  it('odd hex padded', () => {
    expect(bytesToHex(hexToBytes('a'))).toBe('0a')
  })
  it('normalizePeerId trims and lowercases non-64', () => {
    expect(normalizePeerId('  ABC  ')).toBe('abc')
  })
  it('authHeaders uses token', async () => {
    localStorage.setItem('proxi_token', 'tok1')
    const h = await authHeaders()
    expect(h.Authorization).toBe('Bearer tok1')
    expect(h['Content-Type']).toContain('json')
  })
})

describe('NostrDataRelay', () => {
  it('starts disconnected', () => {
    const r = new NostrDataRelay()
    expect(r.getStatus()).toBe('disconnected')
  })
  it('connect without pubkey throws', async () => {
    vi.resetModules()
    const r = new NostrDataRelay()
    await expect(r.connect('')).rejects.toThrow()
  })
})

describe('NostrVPNSignaling', () => {
  it('init without identity throws', async () => {
    const s = new NostrVPNSignaling()
    await expect(s.init('')).rejects.toThrow()
  })
})
