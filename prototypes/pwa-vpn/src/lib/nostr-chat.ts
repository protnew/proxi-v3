import { wsAuthProtocols } from './ws-auth'
/**
 * Nostr Chat Transport — kind:4 DMs over public relays
 * Replaces Go WebSocket for serverless operation
 *
 * NIP-04 E2E: ECDH(secp256k1) → shared secret → AES-256-CBC
 */
import * as secp from '@noble/secp256k1'
import { signEvent, type Identity } from './identity'

const bytesToHex = (bytes: Uint8Array | number[]) => Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
const hexToBytes = (hex: string) => {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  return bytes
}

const KIND_DM = 4
// P4: no public relays — DM/presence only via local embedded relay.
// Public relays leak the social graph (who talks to whom) permanently.
function defaultRelays(): string[] {
  const loc = typeof window !== 'undefined' ? window.location : null
  if (!loc) return ['ws://127.0.0.1:8090/nostr']
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  // /nostr requires JWT (P9) — WS cannot send Authorization header
  return [`${proto}//${loc.host}/nostr`]
}

export interface NostrMessage {
  id: string
  from: string
  to: string
  text: string
  timestamp: number
}

type MessageCallback = (msg: NostrMessage) => void

async function deriveSharedSecret(privKey: string, pubKey: string): Promise<string> {
  // @noble/secp256k1 v2: requires Uint8Array
  const shared = secp.getSharedSecret(hexToBytes(privKey), hexToBytes('02' + pubKey))
  return bytesToHex(shared.slice(1, 33))
}

async function encrypt(text: string, privKey: string, pubKey: string): Promise<string> {
  const sharedSecret = await deriveSharedSecret(privKey, pubKey)
  const key = await crypto.subtle.importKey(
    'raw', hexToBytes(sharedSecret), { name: 'AES-CBC' }, false, ['encrypt']
  )
  const iv = crypto.getRandomValues(new Uint8Array(16))
  const encoded = new TextEncoder().encode(text)
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, key, encoded)
  const ctB64 = btoa(String.fromCharCode(...new Uint8Array(ciphertext)))
  const ivB64 = btoa(String.fromCharCode(...iv))
  return `${ctB64}?iv=${ivB64}`
}

async function decrypt(content: string, privKey: string, pubKey: string): Promise<string> {
  const [ctB64, ivB64] = content.split('?iv=')
  if (!ctB64 || !ivB64) throw new Error('Invalid NIP-04 format')
  const sharedSecret = await deriveSharedSecret(privKey, pubKey)
  const key = await crypto.subtle.importKey(
    'raw', hexToBytes(sharedSecret), { name: 'AES-CBC' }, false, ['decrypt']
  )
  const iv = Uint8Array.from(atob(ivB64), c => c.charCodeAt(0))
  const ct = Uint8Array.from(atob(ctB64), c => c.charCodeAt(0))
  const plain = await crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, ct)
  return new TextDecoder().decode(plain)
}

async function sha256Hex(data: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(data))
  return bytesToHex(new Uint8Array(buf))
}

export class NostrChat {
  private identity: Identity
  // BAG-30: resolve relay URLs lazily at connect time — the JWT may not
  // exist yet at construction (App boots Nostr before Go auth completes).
  private relayResolver: () => string[]
  private connections: WebSocket[] = []
  private messageCallbacks: MessageCallback[] = []
  private connectedCount = 0
  private subId = 'dm-' + Math.random().toString(36).slice(2, 10)

  constructor(identity: Identity, relays?: string[] | (() => string[])) {
    this.identity = identity
    this.relayResolver = typeof relays === 'function' ? relays
      : Array.isArray(relays) ? () => relays
      : defaultRelays
  }

  get connectedRelays(): number { return this.connectedCount }
  get isConnected(): boolean { return this.connectedCount > 0 }

  async connect(): Promise<number> {
    // BAG-30: /nostr is JWT-protected — without a token the handshake is a
    // guaranteed 401 and the close-handler would retry forever.
    const urls = this.relayResolver()
    const tok = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_token') : null
    if (!tok) {
      console.log('[Nostr] No JWT yet — skipping /nostr connect')
      return 0
    }
    const filter = {
      kinds: [KIND_DM],
      '#p': [this.identity.publicKey],
      limit: 50,
    }
    const subMsg = JSON.stringify(['REQ', this.subId, filter])
    const promises = urls.map((_, i) => this.connectRelay(i, subMsg))
    await Promise.allSettled(promises)
    return this.connectedCount
  }

  private connectRelay(idx: number, subMsg: string): Promise<void> {
    // Fresh URL per attempt: reconnects pick up a JWT that appeared later.
    const urls = this.relayResolver()
    const url = urls[Math.min(idx, urls.length - 1)]
    return new Promise((resolve) => {
      try {
        const tok = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_token') : null
        const ws = new WebSocket(url, wsAuthProtocols(tok))
        let settled = false
        ws.onopen = () => {
          ws.send(subMsg)
          this.connectedCount++
          console.log(`[Nostr] Connected to ${url.split('?')[0]} (${this.connectedCount} total)`)
          if (!settled) { settled = true; resolve() }
        }
        ws.onmessage = (e) => this.handleMessage(e.data)
        ws.onerror = () => { if (!settled) { settled = true; resolve() } }
        ws.onclose = () => {
          this.connectedCount = Math.max(0, this.connectedCount - 1)
          setTimeout(() => {
            if (this.messageCallbacks.length > 0) this.connectRelay(idx, subMsg)
          }, 5000)
        }
        this.connections.push(ws)
        setTimeout(() => { if (!settled) { settled = true; resolve() } }, 5000)
      } catch { resolve() }
    })
  }

  onMessage(cb: MessageCallback) { this.messageCallbacks.push(cb) }

  // Presence publish path — module functions have no access to instance sockets
  get openSockets(): WebSocket[] {
    return this.connections.filter(ws => ws.readyState === WebSocket.OPEN)
  }

  async sendDM(recipientPubkey: string, text: string): Promise<boolean> {
    if (!text.trim()) return false
    const encrypted = await encrypt(text, this.identity.privateKey, recipientPubkey)
    const createdAt = Math.floor(Date.now() / 1000)
    const tags = [['p', recipientPubkey]]
    const serialized = JSON.stringify([0, this.identity.publicKey, createdAt, KIND_DM, tags, encrypted])
    const id = await sha256Hex(serialized)
    const sig = await signEvent(
      { pubkey: this.identity.publicKey, created_at: createdAt, kind: KIND_DM, tags, content: encrypted },
      this.identity.privateKey
    )
    const event = { id, pubkey: this.identity.publicKey, created_at: createdAt, kind: KIND_DM, tags, content: encrypted, sig }
    const msg = JSON.stringify(['EVENT', event])
    let sent = false
    for (const ws of this.connections) {
      if (ws.readyState === WebSocket.OPEN) { ws.send(msg); sent = true }
    }
    this.messageCallbacks.forEach(cb => cb({
      id, from: this.identity.publicKey, to: recipientPubkey, text, timestamp: createdAt * 1000,
    }))
    return sent
  }

  private async handleMessage(data: string) {
    try {
      const msg = JSON.parse(data)
      if (msg[0] !== 'EVENT' || !msg[2]) return
      const event = msg[2]
      if (event.kind !== KIND_DM) return
      try {
        const plaintext = await decrypt(event.content, this.identity.privateKey, event.pubkey)
        this.messageCallbacks.forEach(cb => cb({
          id: event.id, from: event.pubkey, to: this.identity.publicKey,
          text: plaintext, timestamp: event.created_at * 1000,
        }))
      } catch { /* not for us */ }
    } catch { /* invalid */ }
  }

  disconnect() {
    this.messageCallbacks = []
    for (const ws of this.connections) { try { ws.close() } catch {} }
    this.connections = []
    this.connectedCount = 0
  }
}


// MSG-105: Presence via Nostr (kind 10002 relay list + periodic ping)
let activeChat: NostrChat | null = null
export function setActiveChat(chat: NostrChat | null) { activeChat = chat }

const PRESENCE_KIND = 34000
type PresenceCallback = (pubkey: string, online: boolean) => void
const presenceCallbacks: PresenceCallback[] = []
const presenceMap = new Map<string, number>()

export function onPresence(cb: PresenceCallback) {
  presenceCallbacks.push(cb)
}

function emitPresence(pubkey: string, online: boolean) {
  presenceCallbacks.forEach(cb => cb(pubkey, online))
}

export async function broadcastPresence(identity: Identity): Promise<void> {
  try {
    const event = {
      kind: PRESENCE_KIND,
      created_at: Math.floor(Date.now() / 1000),
      tags: [],
      content: 'online',
    }
    const signed = await signEvent(event, identity.privateKey)
    for (const ws of activeChat?.openSockets ?? []) {
      ws.send(JSON.stringify(['EVENT', signed]))
    }
  } catch {}
}

export function checkPresenceTimeout() {
  const now = Date.now()
  for (const [pk, lastSeen] of presenceMap) {
    if (now - lastSeen > 120000) { // 2 min
      presenceMap.delete(pk)
      emitPresence(pk, false)
    }
  }
}

// Start presence loop
let presenceInterval: ReturnType<typeof setInterval> | null = null
export function startPresence(identity: Identity) {
  if (presenceInterval) clearInterval(presenceInterval)
  broadcastPresence(identity)
  presenceInterval = setInterval(() => {
    broadcastPresence(identity)
    checkPresenceTimeout()
  }, 60000) // every 60s
}

export function stopPresence() {
  if (presenceInterval) {
    clearInterval(presenceInterval)
    presenceInterval = null
  }
}
