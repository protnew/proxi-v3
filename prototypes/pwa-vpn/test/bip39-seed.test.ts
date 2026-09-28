/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest';
import { createSeedIdentity, deriveFromMnemonic, recoverFromPhrase, saveMnemonic, loadMnemonic } from '../src/lib/bip39-seed';

describe('BIP39 Seed Phrase', () => {
  beforeEach(() => localStorage.clear());

  it('generates 12-word mnemonic', () => {
    const id = createSeedIdentity();
    const words = id.mnemonic.split(' ');
    expect(words.length).toBe(12);
    expect(id.privateKey).toHaveLength(64);
    expect(id.publicKey).toHaveLength(64);
    expect(id.npub).toMatch(/^npub1/);
  });

  it('derives same keys from same mnemonic', () => {
    const id1 = createSeedIdentity();
    const id2 = deriveFromMnemonic(id1.mnemonic);
    expect(id1.privateKey).toBe(id2.privateKey);
    expect(id1.publicKey).toBe(id2.publicKey);
    expect(id1.npub).toBe(id2.npub);
  });

  it('recovers identity from phrase string', () => {
    const id1 = createSeedIdentity();
    // Simulate user typing phrase with extra spaces / caps
    const messy = '  ' + id1.mnemonic.toUpperCase().split(' ').join('  ') + '  ';
    const id2 = recoverFromPhrase(messy);
    expect(id2.privateKey).toBe(id1.privateKey);
  });

  it('save and load mnemonic from localStorage', () => {
    const id = createSeedIdentity();
    saveMnemonic(id.mnemonic);
    const loaded = loadMnemonic();
    expect(loaded).toBe(id.mnemonic);
  });

  it('different mnemonics produce different keys', () => {
    const id1 = createSeedIdentity();
    const id2 = createSeedIdentity();
    expect(id1.privateKey).not.toBe(id2.privateKey);
  });

  it('rejects invalid mnemonic (wrong word count)', () => {
    expect(() => recoverFromPhrase('apple banana cherry')).toThrow();
  });

  it('own phrase (25th word) changes the key; same phrase restores it', () => {
    const id = createSeedIdentity();
    const a = deriveFromMnemonic(id.mnemonic, 'моя осмысленная фраза');
    const b = deriveFromMnemonic(id.mnemonic, 'моя осмысленная фраза');
    const c = deriveFromMnemonic(id.mnemonic, 'другая фраза');
    expect(a.privateKey).toBe(b.privateKey);
    expect(a.privateKey).not.toBe(c.privateKey);
    expect(a.privateKey).not.toBe(id.privateKey);
  });

  it('plain sentence is not a mnemonic (brainwallet rejected)', () => {
    expect(() => recoverFromPhrase('моя осмысленная фраза для входа')).toThrow();
  });
});
