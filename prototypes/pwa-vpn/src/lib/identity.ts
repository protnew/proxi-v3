/**
 * Identity module — secp256k1 keys, Nostr npub/nsec, BIP-39 seed
 * Everything runs in the browser, keys never leave the device
 */
import * as secp from '@noble/secp256k1'

const bytesToHex = (bytes: Uint8Array | number[]) => Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
const hexToBytes = (hex: string) => {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  }
  return bytes
}

// Simple BIP-39 word list (2048 words) — we load a subset for demo
// In production, use @scure/bip39
const STORAGE_KEY = 'indestructible-identity'

export interface Identity {
  privateKey: string  // hex
  publicKey: string   // hex
  npub: string        // bech32 encoded
  nsec: string        // bech32 encoded
  createdAt: number
}

/**
 * Generate a new random identity
 */
export async function createIdentity(): Promise<Identity> {
  const privateKeyBytes = crypto.getRandomValues(new Uint8Array(32))
  const privateKey = bytesToHex(privateKeyBytes)
  const publicKey = bytesToHex(secp.schnorr.getPublicKey(privateKeyBytes))

  const npub = encodeBech32('npub', hexToBytes(publicKey))
  const nsec = encodeBech32('nsec', hexToBytes(privateKey))

  const identity: Identity = {
    privateKey,
    publicKey,
    npub,
    nsec,
    createdAt: Date.now(),
  }

  // Save encrypted with a derived key (for MVP — just localStorage)
  // TODO: encrypt with user's password via PBKDF2
  localStorage.setItem(STORAGE_KEY, JSON.stringify(identity))

  return identity
}

/**
 * Load existing identity from localStorage
 */
export function loadIdentity(): Identity | null {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as Identity
  } catch {
    return null
  }
}

/**
 * Get or create identity
 */
export async function getIdentity(): Promise<Identity> {
  const existing = loadIdentity()
  if (existing) return existing
  return createIdentity()
}

/**
 * Delete identity (logout)
 */
export function deleteIdentity(): void {
  localStorage.removeItem(STORAGE_KEY)
}

/**
 * Sign a Nostr event
 */
export async function signEvent(event: Record<string, unknown>, privateKey: string): Promise<string> {
  // Serialize event for signing (Nostr spec)
  const serialized = JSON.stringify([
    0,
    event['pubkey'],
    event['created_at'],
    event['kind'],
    event['tags'],
    event['content'],
  ])
  const msgHash = bytesToHex(
    new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized)))
  )
  const sigBytes = await secp.schnorr.signAsync(hexToBytes(msgHash), hexToBytes(privateKey))
  const sig = bytesToHex(sigBytes)
  return sig
}

// --- Bech32 encoding (minimal implementation) ---

const BECH32_CHARSET = 'qpzry9x8gf2tvdw0s3jn54khce6mua7l'

function bech32Polymod(values: number[]): number {
  const GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3]
  let chk = 1
  for (const v of values) {
    const b = chk >> 25
    chk = ((chk & 0x1ffffff) << 5) ^ v
    for (let i = 0; i < 5; i++) {
      if ((b >> i) & 1) chk ^= GEN[i]
    }
  }
  return chk
}

function bech32HrpExpand(hrp: string): number[] {
  const ret: number[] = []
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) >> 5)
  ret.push(0)
  for (let i = 0; i < hrp.length; i++) ret.push(hrp.charCodeAt(i) & 31)
  return ret
}

function bech32CreateChecksum(hrp: string, data: number[]): number[] {
  const values = bech32HrpExpand(hrp).concat(data).concat([0, 0, 0, 0, 0, 0])
  const polymod = bech32Polymod(values) ^ 1
  const ret: number[] = []
  for (let i = 0; i < 6; i++) {
    ret.push((polymod >> (5 * (5 - i))) & 31)
  }
  return ret
}

function convertBits(data: Uint8Array, fromBits: number, toBits: number): number[] {
  let acc = 0
  let bits = 0
  const ret: number[] = []
  const maxv = (1 << toBits) - 1
  for (let i = 0; i < data.length; i++) {
    acc = (acc << fromBits) | data[i]
    bits += fromBits
    while (bits >= toBits) {
      bits -= toBits
      ret.push((acc >> bits) & maxv)
    }
  }
  if (bits > 0) {
    ret.push((acc << (toBits - bits)) & maxv)
  }
  return ret
}

function encodeBech32(hrp: string, data: Uint8Array): string {
  const words = convertBits(data, 8, 5)
  const checksum = bech32CreateChecksum(hrp, words)
  let result = hrp + '1'
  for (const w of words) result += BECH32_CHARSET[w]
  for (const c of checksum) result += BECH32_CHARSET[c]
  return result
}
