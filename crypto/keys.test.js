import { describe, it, expect } from 'vitest';
import { generateEd25519KeyPair } from './keys.js';

describe('Ed25519 Key Generation', () => {
  it('should generate a valid key pair', () => {
    const keypair = generateEd25519KeyPair();
    expect(keypair).toHaveProperty('publicKey');
    expect(keypair).toHaveProperty('privateKey');
    expect(keypair.publicKey.length).toBe(32);
    expect(keypair.privateKey.length).toBe(64);
  });
});
