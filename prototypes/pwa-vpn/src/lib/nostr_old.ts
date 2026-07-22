/**
 * Nostr transport layer — handles all relay communication
 * DMs (kind 14 simplified), Channels (NIP-28), metadata, presence
 *
 * Uses nostr-tools for signing, multi-relay for reliability
 */
import { finalizeEvent, generateSecretKey, getPublicKey } from 'nostr-tools/pure'
import { bytesToHex, hexToBytes } from 'nostr-tools/utils'
import { getConversationKey, encrypt as nip44Encrypt, decrypt as nip44Decrypt } from 'nostr-tools/nip44'
import type { Message } from '../stores/messenger'

const RELAYS = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://relay.nostr.band',
]

// Nostr event kinds we use
const KIND_DM = 14              // Direct message (simplified)
const KIND_CHANNEL_CREATE = 40  // Group/channel creation
const KIND_CHANNEL_MSG = 41     // Channel message
const KIND_METADATA = 0         // Profile metadata
const KIND_PRESENCE = 21001     // Custom: online status
const KIND_TYPING = 21002       // Custom: typing indicator
const KIND_FILE_MANIFEST = 21003 // Custom: file metadata
const KIND_CALL_SIGNAL = 21004  // Custom: call signaling

interface RelayConn {
  url: string
  ws: WebSocket
  connected: boolean
  subId: string
}

let connections: RelayConn[] = []
let seckey: Uint8Array = new Uint8Array(0)
let pubkey = ''

// Callbacks — set BEFORE connectRelays so historical events are processed
let onMessageCallback: ((msg: Message) => void) | null = null
let onPresenceCallback: ((pubkey: string, online: boolean) => void) | null = null
let onTypingCallback: ((pubkey: string) => void) | null = null

export function getPubkey() { return pubkey }
export function getSeckey() { return seckey }

export function onMessage(cb: (msg: Message) => void) { onMessageCallback = cb }
export function onPresence(cb: (pk: string, online: boolean) => void) { onPresenceCallback = cb }
export function onTyping(cb: (pk: string) => void) { onTypingCallback = cb }

// --- Identity ---
export function initIdentity(): string {
  // Try load existing FIRST
  const stored = localStorage.getItem('nostr-id')
  if (stored) {
    try {
      const d = JSON.parse(stored)
      if (d.sk && d.pk) {
        seckey = new Uint8Array(d.sk)
        pubkey = d.pk
        console.log('[nostr] Loaded existing identity:', pubkey.slice(0, 8))
        return pubkey
      }
    } catch (e) {
      console.warn('[nostr] Failed to parse stored identity, generating new')
    }
  }

  // Generate new
  seckey = generateSecretKey()
  pubkey = getPublicKey(seckey)
  localStorage.setItem('nostr-id', JSON.stringify({ sk: Array.from(seckey), pk: pubkey }))
  console.log('[nostr] Generated new identity:', pubkey.slice(0, 8))
  return pubkey
}

// --- Relay connections ---
export async function connectRelays(): Promise<number> {
  let connected = 0
  const promises = RELAYS.map(url => connectOneRelay(url))

  const results = await Promise.allSettled(promises)
  connected = results.filter(r => r.status === 'fulfilled' && r.value).length

  console.log(`[nostr] Connected to ${connected}/${RELAYS.length} relays`)
  return connected
}

async function connectOneRelay(url: string): Promise<boolean> {
  return new Promise(resolve => {
    try {
      const ws = new WebSocket(url)
      const timer = setTimeout(() => {
        if (ws.readyState !== WebSocket.OPEN) {
          ws.close()
          resolve(false)
        }
      }, 5000)

      ws.onopen = () => {
        clearTimeout(timer)
        console.log('[nostr] Connected to', url)
        const subId = 'dm-' + Math.random().toString(36).slice(2, 8)
        const conn: RelayConn = { url, ws, connected: true, subId }
        connections.push(conn)

        // Subscribe to DMs and custom events targeting us
        const subMsg = JSON.stringify(['REQ', subId, {
          kinds: [KIND_DM, KIND_PRESENCE, KIND_TYPING, KIND_FILE_MANIFEST, KIND_CALL_SIGNAL],
          '#p': [pubkey],
          limit: 100
        }])
        ws.send(subMsg)

        // Subscribe to metadata (contact names)
        const metaSubId = 'meta-' + Math.random().toString(36).slice(2, 8)
        const metaSub = JSON.stringify(['REQ', metaSubId, { kinds: [KIND_METADATA], limit: 0 }])
        ws.send(metaSub)

        resolve(true)
      }

      ws.onmessage = (e) => handleRelayMessage(e.data, url)
      ws.onerror = () => { clearTimeout(timer) }
      ws.onclose = () => {
        connections = connections.filter(c => c.ws !== ws)
        console.log('[nostr] Disconnected from', url, '— reconnecting in 10s')
        setTimeout(() => connectOneRelay(url), 10000)
      }
    } catch {
      resolve(false)
    }
  })
}

function subscribe(filters: object[]) {
  const subId = 'sub-' + Math.random().toString(36).slice(2, 8)
  const msg = JSON.stringify(['REQ', subId, ...filters])
  let sent = 0
  for (const conn of connections) {
    if (conn.ws.readyState === WebSocket.OPEN) {
      conn.ws.send(msg)
      sent++
    }
  }
  return sent
}

function publish(kind: number, tags: string[][], content: string) {
  const template = {
    pubkey,
    created_at: Math.floor(Date.now() / 1000),
    kind, tags, content,
  }
  const signed = finalizeEvent(template, seckey)
  const msg = JSON.stringify(['EVENT', signed])
  let sent = 0
  for (const conn of connections) {
    if (conn.ws.readyState === WebSocket.OPEN) {
      conn.ws.send(msg)
      sent++
    }
  }
  if (sent === 0) console.warn('[nostr] No relay connected, message not sent!')
  return signed
}

// --- Handle incoming relay messages ---
function handleRelayMessage(data: string, relayUrl: string) {
  try {
    const msg = JSON.parse(data)

    if (msg[0] === 'OK') {
      // console.log('[nostr] Event accepted:', msg[1], msg[2])
      return
    }
    if (msg[0] === 'EOSE') {
      console.log('[nostr] End of stored events for subscription:', msg[1], 'from', relayUrl)
      return
    }
    if (msg[0] === 'NOTICE') {
      console.warn('[nostr] Notice from', relayUrl, ':', msg[1])
      return
    }
    if (msg[0] !== 'EVENT' || !msg[2]) return

    const event = msg[2]

    // Skip own events (they arrive via subscription too if relay echoes)
    if (event.pubkey === pubkey) return

    switch (event.kind) {
      case KIND_DM: {
        const pTag = event.tags.find((t: string[]) => t[0] === 'p')
        if (pTag && pTag[1] === pubkey) {
          const eTag = event.tags.find((t: string[]) => t[0] === 'e')
          // Decrypt content
          let text = event.content || ''
          try {
            text = decryptFrom(event.pubkey, text)
          } catch {
            console.warn('[nostr] Failed to decrypt DM from', event.pubkey.slice(0, 8), '(plaintext or different encryption)')
          }
          const m: Message = {
            id: event.id,
            from: event.pubkey,
            to: pubkey,
            text,
            timestamp: event.created_at * 1000,
            type: 'text',
            read: false,
            replyTo: eTag ? eTag[1] : undefined,
          }
          console.log('[nostr] DM from', event.pubkey.slice(0, 8), ':', m.text.slice(0, 40))
          onMessageCallback?.(m)
        }
        break
      }
      case KIND_PRESENCE: {
        try {
          const d = JSON.parse(event.content)
          onPresenceCallback?.(event.pubkey, d.online ?? true)
        } catch {}
        break
      }
      case KIND_TYPING: {
        onTypingCallback?.(event.pubkey)
        break
      }
      case KIND_FILE_MANIFEST: {
        try {
          const d = JSON.parse(event.content)
          const isImage = d.name && /\.(jpg|jpeg|png|gif|webp|bmp|svg)$/i.test(d.name)
          const isVoice = d.type === 'voice'
          const m: Message = {
            id: event.id,
            from: event.pubkey,
            to: pubkey,
            text: isVoice ? '🎤 Голосовое сообщение' : isImage ? '🖼️ Фото' : `📎 ${d.name || 'file'}`,
            timestamp: event.created_at * 1000,
            type: isVoice ? 'voice' : isImage ? 'image' : 'file',
            fileName: d.name,
            fileSize: d.size,
            fileUrl: d.url,
            voiceDuration: d.duration,
            read: false,
          }
          onMessageCallback?.(m)
        } catch {}
        break
      }
      case KIND_CALL_SIGNAL: {
        try {
          const signal = JSON.parse(event.content)
          import('./calls').then(c => c.handleCallSignal(event.pubkey, signal)).catch(() => {})
          import('./peer-manager').then(pm => pm.handleSignal(event.pubkey, signal)).catch(() => {})
        } catch {}
        break
      }
      case KIND_CHANNEL_MSG: {
        const eTag = event.tags.find((t: string[]) => t[0] === 'e')
        if (eTag) {
          const m: Message = {
            id: event.id,
            from: event.pubkey,
            to: `group:${eTag[1]}`,
            text: event.content || '',
            timestamp: event.created_at * 1000,
            type: 'text',
            read: false,
          }
          onMessageCallback?.(m)
        }
        break
      }
      case KIND_METADATA: {
        try {
          const meta = JSON.parse(event.content)
          if (meta.name) setName(event.pubkey, meta.name)
        } catch {}
        break
      }
    }
  } catch (e) {
    // Silently ignore malformed messages
  }
}

// --- Send DM (NIP-44 encrypted) ---
export function sendDM(targetPubkey: string, text: string, replyToId?: string): Message {
  const tags: string[][] = [['p', targetPubkey]]
  if (replyToId) tags.push(['e', replyToId, '', 'reply'])
  const encrypted = encryptFor(targetPubkey, text)
  publish(KIND_DM, tags, encrypted)
  return {
    id: crypto.randomUUID(),
    from: pubkey,
    to: targetPubkey,
    text,
    timestamp: Date.now(),
    type: 'text',
    read: true,
    replyTo: replyToId,
  }
}

// --- Send file manifest ---
export function sendFileManifest(targetPubkey: string, name: string, size: number, url: string): Message {
  publish(KIND_FILE_MANIFEST, [['p', targetPubkey]], JSON.stringify({ name, size, url }))
  return {
    id: crypto.randomUUID(),
    from: pubkey,
    to: targetPubkey,
    text: `📎 ${name}`,
    timestamp: Date.now(),
    type: 'file',
    fileName: name,
    fileSize: size,
    fileUrl: url,
    read: true,
  }
}

// --- Send voice manifest ---
export function sendVoiceManifest(targetPubkey: string, duration: number, url: string): Message {
  publish(KIND_FILE_MANIFEST, [['p', targetPubkey]], JSON.stringify({ type: 'voice', duration, url }))
  return {
    id: crypto.randomUUID(),
    from: pubkey,
    to: targetPubkey,
    text: '🎤 Голосовое сообщение',
    timestamp: Date.now(),
    type: 'voice',
    fileUrl: url,
    voiceDuration: duration,
    read: true,
  }
}

// --- Presence ---
export function sendPresence(online: boolean) {
  publish(KIND_PRESENCE, [], JSON.stringify({ online }))
}

// --- Typing ---
let typingTimeout: ReturnType<typeof setTimeout> | null = null
export function sendTyping(targetPubkey: string) {
  if (typingTimeout) return // debounce: max 1 per 3s
  publish(KIND_TYPING, [['p', targetPubkey]], '{}')
  typingTimeout = setTimeout(() => { typingTimeout = null }, 3000)
}

// --- Profile metadata ---
export function updateProfile(name: string, about: string, avatar: string) {
  publish(KIND_METADATA, [], JSON.stringify({ name, about, picture: avatar }))
}

// --- Groups (NIP-28) ---
export function createGroup(name: string, about: string): string {
  const event = publish(KIND_CHANNEL_CREATE, [['d', `group-${Date.now()}`]], JSON.stringify({ name, about, picture: '👥' }))
  return event.id
}

export function sendGroupMessage(channelId: string, text: string): Message {
  publish(KIND_CHANNEL_MSG, [['e', channelId, '', 'root']], text)
  return {
    id: crypto.randomUUID(),
    from: pubkey,
    to: `group:${channelId}`,
    text,
    timestamp: Date.now(),
    type: 'text',
    read: true,
  }
}

export function subscribeGroup(channelId: string) {
  subscribe([{ kinds: [KIND_CHANNEL_MSG], '#e': [channelId], limit: 100 }])
}

// --- Call signaling ---
export function sendCallSignal(targetPubkey: string, signal: { type: string; sdp?: string; candidate?: any }) {
  publish(KIND_CALL_SIGNAL, [['p', targetPubkey]], JSON.stringify(signal))
}

export function subscribeCallSignals() {
  subscribe([{ kinds: [KIND_CALL_SIGNAL], '#p': [pubkey], limit: 0 }])
}

// --- NIP-44 Encryption ---
const conversationKeys = new Map<string, Uint8Array>()

function getConvKey(peerPubkey: string): Uint8Array {
  if (conversationKeys.has(peerPubkey)) return conversationKeys.get(peerPubkey)!
  const key = getConversationKey(seckey, peerPubkey)
  conversationKeys.set(peerPubkey, key)
  return key
}

export function encryptFor(peerPubkey: string, plaintext: string): string {
  return nip44Encrypt(plaintext, getConvKey(peerPubkey))
}

export function decryptFrom(peerPubkey: string, ciphertext: string): string {
  return nip44Decrypt(ciphertext, getConvKey(peerPubkey))
}

// --- Name cache ---
const nameCache = new Map<string, string>()
export function getName(pk: string): string {
  if (pk === pubkey) return 'Вы'
  if (nameCache.has(pk)) return nameCache.get(pk)!
  const short = pk.slice(0, 6)
  const name = `User-${short}`
  nameCache.set(pk, name)
  return name
}

export function setName(pk: string, name: string) {
  nameCache.set(pk, name)
}

// --- Debug ---
export function getStatus() {
  return {
    pubkey: pubkey.slice(0, 12) + '...',
    connections: connections.map(c => ({ url: c.url, ready: c.ws.readyState })),
    connectionCount: connections.filter(c => c.ws.readyState === WebSocket.OPEN).length,
  }
}
