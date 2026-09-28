/**
 * Content-addressed file storage (IPFS-compatible path without central server).
 *
 * SL-040/041/042/043/044:
 *  - Hash file → CID (sha256 multihash hex)
 *  - Store blob in IndexedDB (local pin)
 *  - Publish Nostr kind:1063 file metadata event
 *  - Download by CID (local first → public gateway fallback)
 *  - Image preview from object URL / gateway
 *  - Size limit + progress callbacks
 *
 * No Helia dependency in Phase 1 (browser WASM heavy). Gateway = optional fetch.
 * Architecture: decentralized storage, no our servers hold files.
 */

import { encryptDM, decryptDM } from './nip-e2e'

const DB_NAME = 'indestructible-ipfs'
const STORE = 'blobs'
const MAX_FILE_BYTES = 25 * 1024 * 1024 // 25MB product limit
const GATEWAYS = [
  'https://cloudflare-ipfs.com/ipfs/',
  'https://ipfs.io/ipfs/',
  'https://dweb.link/ipfs/',
]

export interface ContentFile {
  cid: string
  name: string
  size: number
  mimeType: string
  encrypted: boolean
  createdAt: number
}

export interface UploadResult {
  cid: string
  name: string
  size: number
  mimeType: string
  previewUrl?: string
  nostrEvent?: Record<string, unknown>
}

function bytesToHex(b: Uint8Array): string {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}

/** CIDv1-like simple content id: bafy... not required — use sha256:<hex> for Phase 1 */
export async function computeCID(data: ArrayBuffer | Uint8Array): Promise<string> {
  // Same-realm copy for jsdom/Node webcrypto interop
  const src = data instanceof ArrayBuffer ? new Uint8Array(data) : data
  const bytes = new Uint8Array(src.byteLength)
  bytes.set(src)
  const hash = await crypto.subtle.digest('SHA-256', bytes)
  return 'sha256:' + bytesToHex(new Uint8Array(hash))
}

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB unavailable'))
      return
    }
    const req = indexedDB.open(DB_NAME, 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) {
        db.createObjectStore(STORE, { keyPath: 'cid' })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error || new Error('IDB open failed'))
  })
}

export async function pinLocal(cid: string, blob: Blob, meta: Omit<ContentFile, 'cid'>): Promise<void> {
  const db = await openDB()
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).put({ cid, blob, ...meta })
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error || new Error('pin failed'))
  })
  db.close()
}

export async function getLocal(cid: string): Promise<{ blob: Blob; meta: ContentFile } | null> {
  try {
    const db = await openDB()
    const row = await new Promise<any>((resolve, reject) => {
      const tx = db.transaction(STORE, 'readonly')
      const req = tx.objectStore(STORE).get(cid)
      req.onsuccess = () => resolve(req.result || null)
      req.onerror = () => reject(req.error)
    })
    db.close()
    if (!row?.blob) return null
    return {
      blob: row.blob as Blob,
      meta: {
        cid: row.cid,
        name: row.name,
        size: row.size,
        mimeType: row.mimeType,
        encrypted: !!row.encrypted,
        createdAt: row.createdAt || 0,
      },
    }
  } catch {
    return null
  }
}

/** Try public IPFS gateways (only works for real IPFS CIDs; sha256: local-only) */
export async function fetchGateway(cid: string): Promise<Blob | null> {
  // Our Phase-1 CIDs are sha256:... — gateways need real IPFS CIDs.
  // Keep hook for future Helia/desktop pin.
  if (cid.startsWith('sha256:')) return null
  for (const g of GATEWAYS) {
    try {
      const r = await fetch(g + cid, { signal: AbortSignal.timeout(8000) })
      if (r.ok) return await r.blob()
    } catch { /* next gateway */ }
  }
  return null
}

export async function downloadByCID(cid: string): Promise<{ blob: Blob; url: string; meta?: ContentFile } | null> {
  const local = await getLocal(cid)
  if (local) {
    return { blob: local.blob, url: URL.createObjectURL(local.blob), meta: local.meta }
  }
  const remote = await fetchGateway(cid)
  if (remote) {
    return { blob: remote, url: URL.createObjectURL(remote) }
  }
  return null
}

/**
 * Upload file: hash → pin local → optional E2E encrypt payload for peer → Nostr metadata.
 */
export async function uploadFile(
  file: File,
  opts: {
    senderPubkey: string
    recipientPubkey?: string
    senderPrivateKey?: string
    onProgress?: (pct: number) => void
    maxBytes?: number
  },
): Promise<UploadResult> {
  const max = opts.maxBytes ?? MAX_FILE_BYTES
  if (file.size > max) {
    throw new Error(`Файл слишком большой: ${Math.round(file.size/1024/1024)}MB (лимит ${Math.round(max/1024/1024)}MB)`)
  }

  opts.onProgress?.(5)
  const buf = await file.arrayBuffer()
  opts.onProgress?.(30)
  const cid = await computeCID(buf)
  opts.onProgress?.(50)

  const blob = new Blob([buf], { type: file.type || 'application/octet-stream' })
  let encrypted = false
  let storeBlob = blob

  // Optional: encrypt whole file for recipient (small files) using NIP-E2E envelope
  if (opts.recipientPubkey && opts.senderPrivateKey && file.size < 200_000) {
    const b64 = btoa(String.fromCharCode(...new Uint8Array(buf)))
    const enc = await encryptDM(b64, opts.senderPrivateKey, opts.recipientPubkey)
    storeBlob = new Blob([enc], { type: 'application/x-nip-e2e' })
    encrypted = true
  }

  await pinLocal(cid, storeBlob, {
    name: file.name,
    size: file.size,
    mimeType: file.type || 'application/octet-stream',
    encrypted,
    createdAt: Date.now(),
  })
  opts.onProgress?.(80)

  const isImage = (file.type || '').startsWith('image/')
  const previewUrl = isImage ? URL.createObjectURL(blob) : undefined

  // Nostr kind:1063 file metadata (NIP-94 style)
  const nostrEvent: Record<string, unknown> = {
    kind: 1063,
    pubkey: opts.senderPubkey,
    created_at: Math.floor(Date.now() / 1000),
    tags: [
      ['url', `ipfs://${cid}`],
      ['m', file.type || 'application/octet-stream'],
      ['size', String(file.size)],
      ['x', cid.replace('sha256:', '')],
      ['name', file.name],
      ...(opts.recipientPubkey ? [['p', opts.recipientPubkey]] : []),
    ],
    content: file.name,
  }

  // Publish best-effort via global Nostr WS if present
  try {
    const w = typeof window !== 'undefined' ? (window as any).__nostrWS : null
    if (w && w.readyState === 1) {
      w.send(JSON.stringify(['EVENT', nostrEvent]))
    }
  } catch { /* non-fatal */ }

  opts.onProgress?.(100)
  return {
    cid,
    name: file.name,
    size: file.size,
    mimeType: file.type || 'application/octet-stream',
    previewUrl,
    nostrEvent,
  }
}

export function formatCIDShort(cid: string): string {
  if (cid.length <= 20) return cid
  return cid.slice(0, 12) + '…' + cid.slice(-6)
}

export { MAX_FILE_BYTES }
