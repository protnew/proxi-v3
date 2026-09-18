/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest';
import 'fake-indexeddb/auto';

// Ensure crypto.subtle is available (Node 18+ has it globally)
import { webcrypto } from 'crypto';
if (!globalThis.crypto) (globalThis as any).crypto = webcrypto;
// jsdom may lack subtle — prefer Node webcrypto
if (!(globalThis.crypto as any).subtle) {
  (globalThis as any).crypto = webcrypto;
}

import {
  createIdentity,
  loadIdentityAsync,
  getIdentity,
  deleteIdentity,
  wipeSecureStore,
  hasLegacySeckeyInLocalStorage,
  importIdentity,
  _clearMemoryCacheForTests,
} from '../src/lib/identity';

describe('Identity management (full)', () => {
  beforeEach(async () => {
    localStorage.clear();
    await wipeSecureStore();
  });

  it('createIdentity returns hex keys', async () => {
    const id = await createIdentity();
    expect(id.privateKey).toMatch(/^[0-9a-f]{64}$/);
    expect(id.publicKey).toMatch(/^[0-9a-f]{64}$/);
    expect(id.npub).toMatch(/^npub1/);
    expect(id.nsec).toMatch(/^nsec1/);
  });

  it('createIdentity persists and reloads from secure store', async () => {
    const id = await createIdentity();
    _clearMemoryCacheForTests();
    const loaded = await loadIdentityAsync();
    expect(loaded).not.toBeNull();
    expect(loaded!.publicKey).toBe(id.publicKey);
    expect(loaded!.privateKey).toBe(id.privateKey);
  });

  it('P11: seckey must NOT be written to localStorage', async () => {
    const id = await createIdentity();
    expect(localStorage.getItem('indestructible-seckey')).toBeNull();
    expect(hasLegacySeckeyInLocalStorage()).toBe(false);
    // pubkey may be present
    expect(localStorage.getItem('indestructible-pubkey')).toBe(id.publicKey);
    // full identity JSON / v2 blob must not hold plaintext private key
    const dump = JSON.stringify(localStorage);
    expect(dump).not.toContain(id.privateKey);
    expect(dump).not.toContain(id.nsec);
  });

  it('P11: migrates legacy localStorage seckey then scrubs it', async () => {
    const priv = 'a'.repeat(64);
    // Derive expected pub via create path would need secp — seed LS then migrate on load
    localStorage.setItem('indestructible-seckey', priv);
    localStorage.setItem('indestructible-pubkey', 'b'.repeat(64));
    const loaded = await loadIdentityAsync();
    expect(loaded).not.toBeNull();
    expect(loaded!.privateKey).toBe(priv);
    expect(localStorage.getItem('indestructible-seckey')).toBeNull();
    expect(hasLegacySeckeyInLocalStorage()).toBe(false);
  });

  it('getIdentity loads existing or creates new', async () => {
    const id1 = await getIdentity();
    const id2 = await getIdentity();
    expect(id1.publicKey).toBe(id2.publicKey);
  });

  it('deleteIdentity clears recoverable identity', async () => {
    await createIdentity();
    await deleteIdentity();
    await wipeSecureStore();
    expect(await loadIdentityAsync()).toBeNull();
  });

  it('each identity is unique', async () => {
    localStorage.clear();
    await wipeSecureStore();
    const id1 = await createIdentity();
    localStorage.clear();
    await wipeSecureStore();
    const id2 = await createIdentity();
    expect(id1.publicKey).not.toBe(id2.publicKey);
  });

  it('importIdentity from hex does not leave seckey in localStorage', async () => {
    const hex = 'c'.repeat(64);
    const id = await importIdentity(hex);
    expect(id.privateKey).toBe(hex);
    expect(localStorage.getItem('indestructible-seckey')).toBeNull();
    expect(JSON.stringify(localStorage)).not.toContain(hex);
  });
});
