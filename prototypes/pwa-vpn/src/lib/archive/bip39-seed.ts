/**
 * BIP39 Seed Phrase — offline identity backup & recovery
 * 12-word mnemonic → secp256k1 keypair
 * No server, no Go backend — fully browser-native
 */
import * as secp from '@noble/secp256k1';
import { generateMnemonic, mnemonicToSeedSync, validateMnemonic } from '@scure/bip39';
import { wordlist } from '@scure/bip39/wordlists/english.js';

export interface SeedIdentity {
  mnemonic: string;       // 12 words
  privateKey: string;     // hex 64
  publicKey: string;      // hex 64 (x-only)
  npub: string;           // bech32 npub1...
}

const STORAGE_KEY = 'proxi_mnemonic';

/** Generate 12-word mnemonic + derive secp256k1 keypair */
export function createSeedIdentity(): SeedIdentity {
  const mnemonic = generateMnemonic(wordlist, 128); // 12 words
  return deriveFromMnemonic(mnemonic);
}

/** Derive keypair from existing mnemonic */
export function deriveFromMnemonic(mnemonic: string): SeedIdentity {
  if (!validateMnemonic(mnemonic, wordlist)) {
    throw new Error('Invalid mnemonic');
  }
  const seed = mnemonicToSeedSync(mnemonic);
  // Take first 32 bytes as private key
  const privKey = seed.slice(0, 32);
  const privateKey = Array.from(privKey).map(b => b.toString(16).padStart(2, '0')).join('');
  const pubPoint = secp.getPublicKey(privKey) as Uint8Array;
  const publicKey = pubPoint.slice(1, 33).reduce((s, b) => s + b.toString(16).padStart(2, '0'), '');
  const npub = encodeBech32('npub', publicKey);
  return { mnemonic, privateKey, publicKey, npub };
}

/** Save mnemonic to localStorage (encrypted by user passphrase optional) */
export function saveMnemonic(mnemonic: string): void {
  localStorage.setItem(STORAGE_KEY, mnemonic);
}

/** Load saved mnemonic */
export function loadMnemonic(): string | null {
  return localStorage.getItem(STORAGE_KEY);
}

/** Recover identity from 12-word phrase */
export function recoverFromPhrase(phrase: string): SeedIdentity {
  const clean = phrase.trim().toLowerCase().replace(/\s+/g, ' ');
  return deriveFromMnemonic(clean);
}

// Bech32 encoder (npub/nsec)
const CHARSET = 'qpzry9x8gf2tvdw0s3jn54khce6mua7l';
function encodeBech32(prefix: string, hex: string): string {
  const data = [];
  for (let i = 0; i < hex.length; i += 2) {
    data.push(parseInt(hex.substr(i, 2), 16));
  }
  // Convert 8-bit to 5-bit
  const bytes = new Uint8Array(data);
  const fiveBit: number[] = [];
  let buffer = 0, bits = 0;
  for (const b of bytes) {
    buffer = (buffer << 8) | b;
    bits += 8;
    while (bits >= 5) {
      fiveBit.push((buffer >> (bits - 5)) & 31);
      bits -= 5;
    }
  }
  if (bits > 0) fiveBit.push((buffer << (5 - bits)) & 31);
  // Checksum
  const chkInput = [0, ...prefix.split('').map(c => c.charCodeAt(0) >> 5)];
  for (const b of bytes) chkInput.push(b & 31);
  chkInput.push(0);
  // polymod
  const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3];
  let pm = 1;
  for (const v of chkInput) {
    const top = pm >> 25;
    pm = ((pm & 0x1ffffff) << 5) ^ v;
    for (let i = 0; i < 5; i++) {
      if ((top >> i) & 1) pm ^= GEN[i];
    }
  }
  const checksum: number[] = [];
  for (let i = 0; i < 6; i++) {
    checksum.push((pm >> (5 * (5 - i))) & 31);
  }
  const combined = [...prefix.split('').map(c => c.charCodeAt(0)), ...fiveBit, ...checksum];
  // Encode using CHARSET and separator
  let result = prefix + '1';
  for (let i = prefix.length + 1; i < combined.length; i++) {
    result += CHARSET[combined[i]];
  }
  return result;
}
