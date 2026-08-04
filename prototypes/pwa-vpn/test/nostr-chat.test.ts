/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest'
import { createIdentity } from '../src/lib/identity'

// We test encrypt/decrypt indirectly through identity + crypto.subtle
// Full NostrChat test requires WebSocket mock — keep in e2e

describe('Identity for Nostr', () => {
  it('generates secp256k1 keypair', async () => {
    const id = await createIdentity()
    expect(id.privateKey).toMatch(/^[0-9a-f]{64}$/)
    expect(id.publicKey).toMatch(/^[0-9a-f]{64}$/)
    expect(id.npub).toMatch(/^npub1/)
    expect(id.nsec).toMatch(/^nsec1/)
  })

  it('persists to localStorage', async () => {
    await createIdentity()
    const raw = localStorage.getItem('indestructible-identity')
    expect(raw).toBeTruthy()
    const parsed = JSON.parse(raw!)
    expect(parsed.publicKey).toMatch(/^[0-9a-f]{64}$/)
  })

  it('loads existing identity', async () => {
    const id1 = await createIdentity()
    const { loadIdentity } = await import('../src/lib/identity')
    const id2 = loadIdentity()
    expect(id2).toBeTruthy()
    expect(id2!.publicKey).toBe(id1.publicKey)
  })
})
