/**
 * NIP-E2E / SL-033 — NIP-44 v2 primary + v1 legacy decrypt.
 *
 * Spec: https://github.com/nostr-protocol/nips/blob/master/44.md
 * Implementation: nostr-tools/nip44 (conversation key + pad + XChaCha20-Poly1305)
 *
 * Wire formats:
 *  - v2 (primary): base64 payload from nostr-tools (version byte 0x02 inside)
 *  - v1 (legacy):  "v1.<iv_b64>.<ct_b64>" AES-GCM from earlier audit fix
 *
 * Table 56 full Double Ratchet remains Go desktop (E2EE-001/002).
 */

import { v2 as nip44 } from 'nostr-tools/nip44'

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.startsWith('0x') ? hex.slice(2) : hex
  if (clean.length % 2 !== 0) throw new Error('bad hex')
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
  return out
}

function bytesToHex(b: Uint8Array): string {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}

function b64encode(bytes: Uint8Array): string {
  let s = ''
  bytes.forEach(x => { s += String.fromCharCode(x) })
  return btoa(s)
}

function b64decode(s: string): Uint8Array {
  const bin = atob(s)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

/** Prefix so receivers can detect NIP-44 without guessing base64 alone */
export const NIP44_PREFIX = 'nip44:'

export function isNip44Payload(text: string): boolean {
  return typeof text === 'string' && text.startsWith(NIP44_PREFIX)
}

export function isLegacyV1Payload(text: string): boolean {
  return typeof text === 'string' && text.startsWith('v1.') && text.split('.').length === 3
}

export function isEncryptedPayload(text: string): boolean {
  return isNip44Payload(text) || isLegacyV1Payload(text)
}

function conversationKey(myPrivateHex: string, theirPublicHex: string): Uint8Array {
  // nostr-tools expects priv as Uint8Array, pubkey as hex string (x-only 64 hex)
  const pk = theirPublicHex.length === 66 && (theirPublicHex.startsWith('02') || theirPublicHex.startsWith('03'))
    ? theirPublicHex.slice(2)
    : theirPublicHex
  return nip44.utils.getConversationKey(hexToBytes(myPrivateHex), pk)
}

/**
 * Encrypt with NIP-44 v2. Returns "nip44:<base64payload>".
 */
export async function encryptDM(
  plaintext: string,
  myPrivateHex: string,
  theirPublicHex: string,
): Promise<string> {
  const key = conversationKey(myPrivateHex, theirPublicHex)
  const payload = nip44.encrypt(plaintext, key)
  return NIP44_PREFIX + payload
}

/**
 * Decrypt NIP-44 v2 or legacy v1 AES-GCM.
 */
export async function decryptDM(
  payload: string,
  myPrivateHex: string,
  theirPublicHex: string,
): Promise<string | null> {
  try {
    if (isNip44Payload(payload)) {
      const key = conversationKey(myPrivateHex, theirPublicHex)
      return nip44.decrypt(payload.slice(NIP44_PREFIX.length), key)
    }
    if (isLegacyV1Payload(payload)) {
      return decryptLegacyV1(payload, myPrivateHex, theirPublicHex)
    }
    return null
  } catch {
    return null
  }
}

// ---------- legacy v1 (AES-GCM + HKDF) for messages already in flight ----------

async function hkdfSha256(ikm: Uint8Array, info: string, length = 32): Promise<Uint8Array> {
  const baseKey = await crypto.subtle.importKey('raw', ikm as BufferSource, 'HKDF', false, ['deriveBits'])
  const bits = await crypto.subtle.deriveBits(
    {
      name: 'HKDF',
      hash: 'SHA-256',
      salt: new Uint8Array(32),
      info: new TextEncoder().encode(info),
    },
    baseKey,
    length * 8,
  )
  return new Uint8Array(bits)
}

async function deriveLegacyAesKey(myPrivateHex: string, theirPublicHex: string): Promise<CryptoKey> {
  const secp = await import('@noble/secp256k1')
  const priv = hexToBytes(myPrivateHex)
  let pub = hexToBytes(theirPublicHex)
  if (pub.length === 32) {
    const withPrefix = new Uint8Array(33)
    withPrefix[0] = 0x02
    withPrefix.set(pub, 1)
    pub = withPrefix
  }
  const shared = secp.getSharedSecret(priv, pub, true)
  const sharedU8 = shared instanceof Uint8Array ? shared : new Uint8Array(shared as ArrayLike<number>)
  const ikm = sharedU8.length >= 33 ? sharedU8.slice(1, 33) : sharedU8.slice(0, 32)
  const myPub = secp.schnorr.getPublicKey(priv)
  const myPubHex = bytesToHex(myPub instanceof Uint8Array ? myPub : new Uint8Array(myPub as ArrayLike<number>))
  const peers = [myPubHex.toLowerCase(), theirPublicHex.toLowerCase()].sort()
  const infoStable = `indestructible-nip-e2e-v1|${peers[0]}|${peers[1]}`
  const keyBytes = await hkdfSha256(ikm, infoStable, 32)
  return crypto.subtle.importKey('raw', keyBytes as BufferSource, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'])
}

async function decryptLegacyV1(payload: string, myPrivateHex: string, theirPublicHex: string): Promise<string | null> {
  try {
    const parts = payload.split('.')
    if (parts.length !== 3) return null
    const iv = b64decode(parts[1])
    const ct = b64decode(parts[2])
    const key = await deriveLegacyAesKey(myPrivateHex, theirPublicHex)
    const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct as BufferSource)
    return new TextDecoder().decode(pt)
  } catch {
    return null
  }
}
