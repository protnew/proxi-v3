/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest';

// Ensure crypto.subtle is available (Node 18+ has it globally)
import { webcrypto } from 'crypto';
if (!globalThis.crypto) (globalThis as any).crypto = webcrypto;

import { createIdentity, loadIdentity, getIdentity, deleteIdentity, signEvent } from '../src/lib/identity';

describe('Identity management (full)', () => {
  beforeEach(() => localStorage.clear());

  it('createIdentity returns hex keys', async () => {
    const id = await createIdentity();
    expect(id.privateKey).toMatch(/^[0-9a-f]{64}$/);
    expect(id.publicKey).toMatch(/^[0-9a-f]{64}$/);
    expect(id.npub).toMatch(/^npub1/);
    expect(id.nsec).toMatch(/^nsec1/);
  });

  it('createIdentity stores in localStorage', async () => {
    const id = await createIdentity();
    const loaded = loadIdentity();
    expect(loaded).not.toBeNull();
    expect(loaded!.publicKey).toBe(id.publicKey);
  });

  it('getIdentity loads existing or creates new', async () => {
    const id1 = await getIdentity();
    const id2 = await getIdentity();
    expect(id1.publicKey).toBe(id2.publicKey);
  });

  it('deleteIdentity clears localStorage', async () => {
    await createIdentity();
    deleteIdentity();
    expect(loadIdentity()).toBeNull();
  });

  it('each identity is unique', async () => {
    localStorage.clear();
    const id1 = await createIdentity();
    localStorage.clear();
    const id2 = await createIdentity();
    expect(id1.publicKey).not.toBe(id2.publicKey);
  });
});
