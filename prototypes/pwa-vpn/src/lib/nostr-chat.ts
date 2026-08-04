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
const DEFAULT_RELAYS = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://relay.nostr.band',
]

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
  private relays: string[]
  private connections: WebSocket[] = []
  private messageCallbacks: MessageCallback[] = []
  private connectedCount = 0
  private subId = 'dm-' + Math.random().toString(36).slice(2, 10)

  constructor(identity: Identity, relays: string[] = DEFAULT_RELAYS) {
    this.identity = identity
    this.relays = relays
  }

  get connectedRelays(): number { return this.connectedCount }
  get isConnected(): boolean { return this.connectedCount > 0 }

  async connect(): Promise<number> {
    const filter = {
      kinds: [KIND_DM],
      '#p': [this.identity.publicKey],
      limit: 50,
    }
    const subMsg = JSON.stringify(['REQ', this.subId, filter])
    const promises = this.relays.map(url => this.connectRelay(url, subMsg))
    await Promise.allSettled(promises)
    return this.connectedCount
  }

  private connectRelay(url: string, subMsg: string): Promise<void> {
    return new Promise((resolve) => {
      try {
        const ws = new WebSocket(url)
        let settled = false
        ws.onopen = () => {
          ws.send(subMsg)
          this.connectedCount++
          console.log(`[Nostr] Connected to ${url} (${this.connectedCount} total)`)
          if (!settled) { settled = true; resolve() }
        }
        ws.onmessage = (e) => this.handleMessage(e.data)
        ws.onerror = () => { if (!settled) { settled = true; resolve() } }
        ws.onclose = () => {
          this.connectedCount = Math.max(0, this.connectedCount - 1)
          setTimeout(() => {
            if (this.messageCallbacks.length > 0) this.connectRelay(url, subMsg)
          }, 5000)
        }
        this.connections.push(ws)
        setTimeout(() => { if (!settled) { settled = true; resolve() } }, 5000)
      } catch { resolve() }
    })
  }

  onMessage(cb: MessageCallback) { this.messageCallbacks.push(cb) }

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
