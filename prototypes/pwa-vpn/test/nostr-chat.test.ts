/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest'
import 'fake-indexeddb/auto'
import { webcrypto } from 'crypto'
if (!(globalThis as any).crypto?.subtle) (globalThis as any).crypto = webcrypto

import { createIdentity, loadIdentityAsync, wipeSecureStore, _clearMemoryCacheForTests } from '../src/lib/identity'

describe('Identity for Nostr', () => {
  beforeEach(async () => {
    localStorage.clear()
    await wipeSecureStore()
  })

  it('generates secp256k1 keypair', async () => {
    const id = await createIdentity()
    expect(id.privateKey).toMatch(/^[0-9a-f]{64}$/)
    expect(id.publicKey).toMatch(/^[0-9a-f]{64}$/)
    expect(id.npub).toMatch(/^npub1/)
    expect(id.nsec).toMatch(/^nsec1/)
  })

  it('P11: persists to IndexedDB, not localStorage seckey', async () => {
    const id = await createIdentity()
    expect(localStorage.getItem('indestructible-seckey')).toBeNull()
    expect(localStorage.getItem('indestructible-identity-v2')).toBeNull()
    expect(localStorage.getItem('indestructible-pubkey')).toBe(id.publicKey)
    _clearMemoryCacheForTests()
    const loaded = await loadIdentityAsync()
    expect(loaded).toBeTruthy()
    expect(loaded!.publicKey).toBe(id.publicKey)
    expect(loaded!.privateKey).toBe(id.privateKey)
  })

  it('loads existing identity', async () => {
    const id1 = await createIdentity()
    const id2 = await loadIdentityAsync()
    expect(id2).toBeTruthy()
    expect(id2!.publicKey).toBe(id1.publicKey)
  })
})
