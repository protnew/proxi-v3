/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest'
import { createIdentity, loadIdentityAsync } from '../src/lib/identity'

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
    const id = await createIdentity()
    // v2 encrypted storage (audit fix — no plaintext private key)
    const enc = localStorage.getItem('indestructible-identity-v2')
    expect(enc).toBeTruthy()
    const loaded = await loadIdentityAsync()
    expect(loaded).toBeTruthy()
    expect(loaded!.publicKey).toBe(id.publicKey)
    expect(loaded!.publicKey).toMatch(/^[0-9a-f]{64}$/)
  })

  it('loads existing identity', async () => {
    const id1 = await createIdentity()
    const { loadIdentityAsync } = await import('../src/lib/identity')
    const id2 = await loadIdentityAsync()
    expect(id2).toBeTruthy()
    expect(id2!.publicKey).toBe(id1.publicKey)
  })
})
