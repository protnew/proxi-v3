/**
 * Identity module — secp256k1 keys, Nostr npub/nsec, BIP-39 seed
 * Everything runs in the browser, keys never leave the device.
 *
 * P11: private key material is NOT stored in localStorage.
 *   - Non-extractable AES-GCM CryptoKey lives in IndexedDB
 *   - Encrypted identity blob (AES-GCM) also in IndexedDB
 *   - Only pubkey may remain in localStorage for sync boot
 *   - Legacy indestructible-seckey / identity-v2 are migrated then removed
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

const STORAGE_KEY = 'indestructible-identity'
const STORAGE_ENC = 'indestructible-identity-v2'
const DEVICE_KEY = 'indestructible-device-key'
const SECKEY_LS = 'indestructible-seckey'
const PUBKEY_LS = 'indestructible-pubkey'

const IDB_NAME = 'indestructible-secure'
const IDB_STORE = 'keys'
const IDB_WRAP_KEY = 'wrap-key-v1'
const IDB_IDENTITY = 'identity-v3'

export interface Identity {
  privateKey: string  // hex — held in memory only after unlock
  publicKey: string   // hex
  npub: string        // bech32 encoded
  nsec: string        // bech32 encoded
  createdAt: number
}

/** In-memory cache after create/load — never written to localStorage. */
let memoryCache: Identity | null = null

/** @internal test helper — simulates page reload without wiping IDB */
export function _clearMemoryCacheForTests(): void {
  memoryCache = null
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

  await persistIdentitySecure(identity)
  return identity
}

// ---------- IndexedDB + non-extractable wrapping key (P11) ----------

function openSecureDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('indexedDB unavailable'))
      return
    }
    const req = indexedDB.open(IDB_NAME, 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(IDB_STORE)) {
        db.createObjectStore(IDB_STORE)
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error || new Error('idb open failed'))
  })
}

function idbGet<T>(key: string): Promise<T | undefined> {
  return openSecureDB().then(
    (db) =>
      new Promise((resolve, reject) => {
        const tx = db.transaction(IDB_STORE, 'readonly')
        const req = tx.objectStore(IDB_STORE).get(key)
        req.onsuccess = () => resolve(req.result as T | undefined)
        req.onerror = () => reject(req.error)
        tx.oncomplete = () => db.close()
      }),
  )
}

function idbSet(key: string, value: unknown): Promise<void> {
  return openSecureDB().then(
    (db) =>
      new Promise((resolve, reject) => {
        const tx = db.transaction(IDB_STORE, 'readwrite')
        tx.objectStore(IDB_STORE).put(value, key)
        tx.oncomplete = () => {
          db.close()
          resolve()
        }
        tx.onerror = () => reject(tx.error)
      }),
  )
}

function idbDel(key: string): Promise<void> {
  return openSecureDB().then(
    (db) =>
      new Promise((resolve, reject) => {
        const tx = db.transaction(IDB_STORE, 'readwrite')
        tx.objectStore(IDB_STORE).delete(key)
        tx.oncomplete = () => {
          db.close()
          resolve()
        }
        tx.onerror = () => reject(tx.error)
      }),
  )
}

/** Non-extractable AES-GCM wrapping key stored in IndexedDB (structured clone). */
async function getOrCreateWrapKey(): Promise<CryptoKey> {
  const existing = await idbGet<CryptoKey>(IDB_WRAP_KEY)
  if (existing) return existing
  const key = await crypto.subtle.generateKey(
    { name: 'AES-GCM', length: 256 },
    false, // non-extractable
    ['encrypt', 'decrypt'],
  )
  await idbSet(IDB_WRAP_KEY, key)
  return key
}

type IdentityPackV3 = { v: 3; iv: string; ct: string }

async function persistIdentitySecure(identity: Identity): Promise<void> {
  memoryCache = identity
  try {
    const key = await getOrCreateWrapKey()
    const iv = crypto.getRandomValues(new Uint8Array(12))
    const pt = new TextEncoder().encode(JSON.stringify(identity))
    const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, pt)
    const pack: IdentityPackV3 = {
      v: 3,
      iv: bytesToHex(iv),
      ct: btoa(String.fromCharCode(...new Uint8Array(ct))),
    }
    await idbSet(IDB_IDENTITY, pack)
    // Pubkey only in localStorage (public material, sync boot helpers)
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(PUBKEY_LS, identity.publicKey)
        // P11: never persist seckey; scrub legacy leftovers
        localStorage.removeItem(SECKEY_LS)
        localStorage.removeItem(STORAGE_KEY)
        localStorage.removeItem(STORAGE_ENC)
        localStorage.removeItem(DEVICE_KEY)
      }
    } catch { /* SSR / quota */ }
  } catch (e) {
    console.warn('[identity] secure persist failed', e)
    throw e
  }
}

/** Legacy v2: device secret in localStorage + AES blob — migrate then scrub. */
async function migrateLegacyLocalStorage(): Promise<Identity | null> {
  if (typeof localStorage === 'undefined') return null

  // Plain seckey (worst case) — rebuild identity and move to IDB
  const legacySec = localStorage.getItem(SECKEY_LS)
  const legacyPub = localStorage.getItem(PUBKEY_LS)
  if (legacySec && /^[0-9a-fA-F]{64}$/.test(legacySec)) {
    const privateKeyBytes = hexToBytes(legacySec.toLowerCase())
    const publicKey = legacyPub && legacyPub.length === 64
      ? legacyPub.toLowerCase()
      : bytesToHex(secp.schnorr.getPublicKey(privateKeyBytes))
    const identity: Identity = {
      privateKey: legacySec.toLowerCase(),
      publicKey,
      npub: encodeBech32('npub', hexToBytes(publicKey)),
      nsec: encodeBech32('nsec', privateKeyBytes),
      createdAt: Date.now(),
    }
    await persistIdentitySecure(identity)
    return identity
  }

  // v2 encrypted blob in localStorage
  const enc = localStorage.getItem(STORAGE_ENC)
  if (enc) {
    try {
      const pack = JSON.parse(enc) as { v: number; iv: string; ct: string }
      const deviceKey = await getLegacyDeviceKey()
      const iv = hexToBytes(pack.iv)
      const ctBin = atob(pack.ct)
      const ct = new Uint8Array(ctBin.length)
      for (let i = 0; i < ctBin.length; i++) ct[i] = ctBin.charCodeAt(i)
      const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, deviceKey, ct)
      const identity = JSON.parse(new TextDecoder().decode(pt)) as Identity
      await persistIdentitySecure(identity)
      return identity
    } catch (e) {
      console.warn('[identity] v2 migrate failed', e)
    }
  }

  // v1 plaintext JSON
  const legacy = localStorage.getItem(STORAGE_KEY)
  if (legacy) {
    try {
      const identity = JSON.parse(legacy) as Identity
      if (identity?.privateKey) {
        await persistIdentitySecure(identity)
        return identity
      }
    } catch { /* */ }
  }

  return null
}

async function getLegacyDeviceKey(): Promise<CryptoKey> {
  let raw = localStorage.getItem(DEVICE_KEY)
  let bytes: Uint8Array
  if (!raw) {
    bytes = crypto.getRandomValues(new Uint8Array(32))
  } else {
    bytes = hexToBytes(raw)
  }
  const base = await crypto.subtle.importKey('raw', bytes as BufferSource, 'PBKDF2', false, ['deriveKey'])
  return crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: new TextEncoder().encode('indestructible-identity-v2'),
      iterations: 100_000,
      hash: 'SHA-256',
    },
    base,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  )
}

async function loadFromSecureStore(): Promise<Identity | null> {
  try {
    const pack = await idbGet<IdentityPackV3>(IDB_IDENTITY)
    if (!pack || pack.v !== 3) return null
    const key = await getOrCreateWrapKey()
    const iv = hexToBytes(pack.iv)
    const ctBin = atob(pack.ct)
    const ct = new Uint8Array(ctBin.length)
    for (let i = 0; i < ctBin.length; i++) ct[i] = ctBin.charCodeAt(i)
    const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct)
    return JSON.parse(new TextDecoder().decode(pt)) as Identity
  } catch (e) {
    console.warn('[identity] secure load failed', e)
    return null
  }
}

/**
 * Sync load — cannot unlock IndexedDB; returns memory cache only.
 * Prefer loadIdentityAsync().
 */
export function loadIdentity(): Identity | null {
  return memoryCache
}

/** Public key from localStorage (safe) — used by sync boot paths. */
export function loadStoredPubkey(): string | null {
  try {
    if (typeof localStorage === 'undefined') return null
    return localStorage.getItem(PUBKEY_LS)
  } catch {
    return null
  }
}

/** True if a legacy plaintext seckey is still sitting in localStorage (should be scrubbed). */
export function hasLegacySeckeyInLocalStorage(): boolean {
  try {
    if (typeof localStorage === 'undefined') return false
    const v = localStorage.getItem(SECKEY_LS)
    return !!(v && v.length > 0)
  } catch {
    return false
  }
}

/** Async load — IndexedDB v3, with one-shot migration from localStorage legacy. */
export async function loadIdentityAsync(): Promise<Identity | null> {
  if (memoryCache) return memoryCache
  let id = await loadFromSecureStore()
  if (!id) {
    id = await migrateLegacyLocalStorage()
  } else {
    // Scrub any leftover plaintext seckey even when IDB already has the key
    try {
      localStorage?.removeItem(SECKEY_LS)
      localStorage?.removeItem(STORAGE_KEY)
      localStorage?.removeItem(STORAGE_ENC)
      localStorage?.removeItem(DEVICE_KEY)
    } catch { /* */ }
  }
  if (id) memoryCache = id
  return id
}

/**
 * Get or create identity
 */
export async function getIdentity(): Promise<Identity> {
  const existing = await loadIdentityAsync()
  if (existing) return existing
  return createIdentity()
}

/**
 * Delete identity (logout) — clears IDB identity blob + LS pubkey + memory.
 * Wrapping key is retained (device binding); use wipeSecureStore() for full wipe.
 */
export async function deleteIdentity(): Promise<void> {
  memoryCache = null
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem(STORAGE_KEY)
      localStorage.removeItem(STORAGE_ENC)
      localStorage.removeItem(DEVICE_KEY)
      localStorage.removeItem(SECKEY_LS)
      localStorage.removeItem(PUBKEY_LS)
    }
  } catch { /* SSR */ }
  try {
    await idbDel(IDB_IDENTITY)
  } catch { /* */ }
}

/** Full wipe including non-extractable wrap key (factory reset). */
export async function wipeSecureStore(): Promise<void> {
  memoryCache = null
  deleteIdentity()
  try {
    await idbDel(IDB_WRAP_KEY)
    await idbDel(IDB_IDENTITY)
  } catch { /* */ }
}

/**
 * Sign a Nostr event
 */
export async function signEvent(event: Record<string, unknown>, privateKey: string): Promise<string> {
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

/** Decode bech32 npub/nsec → bytes */
function decodeBech32(bech: string): { hrp: string; data: Uint8Array } {
  const lower = bech.trim().toLowerCase()
  const pos = lower.lastIndexOf('1')
  if (pos < 1) throw new Error('invalid bech32')
  const hrp = lower.slice(0, pos)
  const dataPart = lower.slice(pos + 1)
  const words: number[] = []
  for (const ch of dataPart) {
    const v = BECH32_CHARSET.indexOf(ch)
    if (v < 0) throw new Error('invalid bech32 char')
    words.push(v)
  }
  if (words.length < 7) throw new Error('bech32 too short')
  const dataWords = words.slice(0, -6)
  let acc = 0, bits = 0
  const bytes: number[] = []
  const maxv = 255
  for (const w of dataWords) {
    acc = (acc << 5) | w
    bits += 5
    while (bits >= 8) {
      bits -= 8
      bytes.push((acc >> bits) & maxv)
    }
  }
  return { hrp, data: new Uint8Array(bytes) }
}

/**
 * ONB-000: Import identity from nsec1… hex private key.
 */
export async function importIdentity(input: string): Promise<Identity> {
  const raw = input.trim()
  let privateKeyHex: string
  if (raw.startsWith('nsec1') || raw.startsWith('NSEC1')) {
    const { hrp, data } = decodeBech32(raw)
    if (hrp !== 'nsec') throw new Error('expected nsec')
    if (data.length < 32) throw new Error('nsec payload too short')
    privateKeyHex = bytesToHex(data.slice(0, 32))
  } else if (/^[0-9a-fA-F]{64}$/.test(raw)) {
    privateKeyHex = raw.toLowerCase()
  } else {
    throw new Error('unsupported key format (use nsec1… or 64-hex)')
  }
  const privateKeyBytes = hexToBytes(privateKeyHex)
  const publicKey = bytesToHex(secp.schnorr.getPublicKey(privateKeyBytes))
  const identity: Identity = {
    privateKey: privateKeyHex,
    publicKey,
    npub: encodeBech32('npub', hexToBytes(publicKey)),
    nsec: encodeBech32('nsec', privateKeyBytes),
    createdAt: Date.now(),
  }
  await persistIdentitySecure(identity)
  return identity
}
