/**
 * NIP-E2E — browser DM encryption (Phase 1 intermediate).
 *
 * Table 56 winner = Double Ratchet + CRDT (194) → Go/desktop E2EE-001/002.
 * This module is Phase-1 PWA path:
 *   ECDH (secp256k1) → HKDF-SHA256 → AES-256-GCM
 *
 * Fixes audit CRITICAL #5: never use raw ECDH bytes as AES key.
 * Format: v1.<nonce_b64>.<ciphertext_b64>
 * Conversation key is derived per (sender,recipient) pair via HKDF info domain.
 */

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

/** Normalize shared secret: take x-only if uncompressed point (0x04||X||Y) */
function normalizeShared(shared: Uint8Array): Uint8Array {
  if (shared.length === 33 && (shared[0] === 0x02 || shared[0] === 0x03)) {
    return shared.slice(1) // compressed → X
  }
  if (shared.length === 65 && shared[0] === 0x04) {
    return shared.slice(1, 33) // uncompressed → X
  }
  if (shared.length === 32) return shared
  // fallback: hash whatever we got
  return shared.slice(0, Math.min(32, shared.length))
}

/**
 * HKDF-SHA256 extract+expand → 32-byte AES key.
 * info binds keys to protocol domain (prevents cross-protocol reuse).
 */
async function hkdfSha256(ikm: Uint8Array, info: string, length = 32): Promise<Uint8Array> {
  const baseKey = await crypto.subtle.importKey('raw', ikm as BufferSource, 'HKDF', false, ['deriveBits'])
  const bits = await crypto.subtle.deriveBits(
    {
      name: 'HKDF',
      hash: 'SHA-256',
      salt: new Uint8Array(32), // fixed zero salt OK when ikm is high-entropy ECDH
      info: new TextEncoder().encode(info),
    },
    baseKey,
    length * 8,
  )
  return new Uint8Array(bits)
}

async function deriveAesKey(
  myPrivateHex: string,
  theirPublicHex: string,
  _direction: 'send' | 'recv' = 'send',
): Promise<CryptoKey> {
  // Dynamic import — keeps unit tests light if noble not needed
  const secp = await import('@noble/secp256k1')
  const priv = hexToBytes(myPrivateHex)
  let pub = hexToBytes(theirPublicHex)
  // Nostr x-only pubkeys are 32 bytes — prepend 0x02 for compressed
  if (pub.length === 32) {
    const withPrefix = new Uint8Array(33)
    withPrefix[0] = 0x02
    withPrefix.set(pub, 1)
    pub = withPrefix
  }
  const shared = secp.getSharedSecret(priv, pub, true) // compressed point
  const ikm = normalizeShared(shared instanceof Uint8Array ? shared : new Uint8Array(shared as ArrayLike<number>))

  // Domain separation: sorted pair so both sides derive same conversation key
  // Conversation id from both public keys sorted (stable both directions)
  // We don't have our pub here easily — use their pub + direction salt via info
  const myPub = secp.schnorr.getPublicKey(priv)
  const myPubHex = bytesToHex(myPub instanceof Uint8Array ? myPub : new Uint8Array(myPub as ArrayLike<number>))
  const peers = [myPubHex.toLowerCase(), theirPublicHex.toLowerCase()].sort()
  const infoStable = `indestructible-nip-e2e-v1|${peers[0]}|${peers[1]}`
  const keyBytes = await hkdfSha256(ikm, infoStable, 32)
  return crypto.subtle.importKey('raw', keyBytes as BufferSource, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt'])
}

export async function encryptDM(
  plaintext: string,
  myPrivateHex: string,
  theirPublicHex: string,
): Promise<string> {
  const key = await deriveAesKey(myPrivateHex, theirPublicHex, 'send')
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const ct = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    new TextEncoder().encode(plaintext),
  )
  return `v1.${b64encode(iv)}.${b64encode(new Uint8Array(ct))}`
}

export async function decryptDM(
  payload: string,
  myPrivateHex: string,
  theirPublicHex: string,
): Promise<string | null> {
  try {
    if (!payload.startsWith('v1.')) return null
    const parts = payload.split('.')
    if (parts.length !== 3) return null
    const iv = b64decode(parts[1])
    const ct = b64decode(parts[2])
    const key = await deriveAesKey(myPrivateHex, theirPublicHex, 'recv')
    const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct as BufferSource)
    return new TextDecoder().decode(pt)
  } catch {
    return null
  }
}

export function isEncryptedPayload(text: string): boolean {
  return typeof text === 'string' && text.startsWith('v1.') && text.split('.').length === 3
}
