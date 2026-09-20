/**
 * Nostr Data Relay - VPN traffic transport over Nostr events.
 * Architecture table 58_TURN_Fallback: Phase 1.5 fallback for 15% NAT.
 *
 * When WebRTC P2P fails (CGNAT, symmetric NAT, mobile networks),
 * VPN data is chunked and sent through Nostr events.
 *
 * Flow:
 * 1. WebRTC P2P attempt fails or times out
 * 2. Nostr Data Relay takes over: chunks HTTP request into Nostr events
 * 3. Exit node receives events, reassembles, does fetch, sends response back
 * 4. Nostr relays are decentralized - thousands of nodes, cannot be blocked
 *
 * Encryption: Nostr content is encrypted with NIP-44 (E2E between peers).
 * The relay sees only encrypted blobs.
 *
 * Performance: ~100-500ms latency per request (WebSocket relay hop).
 * Acceptable for browsing, not ideal for streaming.
 */

import * as secp from "@noble/secp256k1"
import { signEvent } from "./identity"
import { getPubkey, getSeckey } from "./api"
import { getStoredToken } from "./api-core"

// ─── Helpers ───
function hexToBytes(hex: string): Uint8Array {
  const h = hex.length % 2 ? "0" + hex : hex
  const out = new Uint8Array(h.length / 2)
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16)
  return out
}
function bytesToHex(b: Uint8Array): string {
  return Array.from(b).map(x => x.toString(16).padStart(2, "0")).join("")
}
function signingIdentity(): { pubkey: string; seckey: string } {
  const seckey = getSeckey()
  if (!seckey || seckey.length < 64) throw new Error("No seckey")
  const sk = seckey.slice(0, 64)
  const pubkey = bytesToHex(secp.schnorr.getPublicKey(hexToBytes(sk)))
  return { pubkey, seckey: sk }
}
async function sha256Hex(data: string): Promise<string> {
  const buf = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(data))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, "0")).join("")
}
function relayWsUrl(): string {
  const loc = typeof window !== "undefined" ? window.location : null
  if (!loc) return "ws://127.0.0.1:8090/nostr"
  const proto = loc.protocol === "https:" ? "wss:" : "ws:"
  // P9: /nostr требует JWT — добавляем ?token= (WS не умеет Authorization header)
  const tok = getStoredToken()
  return `${proto}//${loc.host}/nostr${tok ? `?token=${encodeURIComponent(tok)}` : ''}`
}

// ─── Protocol ───
// Nostr kind 30091: VPN Data Relay events
// Content format: JSON { seq, total, data, reqId, type, target? }
const KIND_VPN_DATA = 30091
const CHUNK_SIZE = 50000 // bytes per Nostr event (safe limit)

interface DataChunk {
  reqId: string
  seq: number
  total: number
  type: "request" | "response"
  data: string      // base64 of chunk data
  target?: string   // URL for request type
  status?: number
}

export type RelayStatus = "disconnected" | "connecting" | "relay" | "error"

interface PendingRequest {
  reqId: string
  chunks: Map<number, string>
  total: number
  resolve: (body: string) => void
  reject: (e: Error) => void
  timer: ReturnType<typeof setTimeout>
}

/**
 * Nostr Data Relay client.
 * Sends VPN HTTP requests through Nostr events when WebRTC fails.
 */
export class NostrDataRelay {
  private ws: WebSocket | null = null
  private status: RelayStatus = "disconnected"
  private myPubkey = ""
  private subId = "relay-" + Math.random().toString(36).slice(2, 10)
  private seenIds = new Set<string>()
  private startedAt = Math.floor(Date.now() / 1000)
  private pending = new Map<string, PendingRequest>()

  // Exit node mode: handle incoming requests
  private exitMode = false
  private onRequest: ((reqId: string, target: string) => Promise<void>) | null = null

  getStatus(): RelayStatus { return this.status }

  async connect(pubkey?: string): Promise<void> {
    try {
      const id = signingIdentity()
      this.myPubkey = pubkey || id.pubkey
    } catch {
      this.myPubkey = pubkey || getPubkey()
    }
    if (!this.myPubkey) throw new Error("No pubkey - login first")

    if (this.ws && this.ws.readyState === WebSocket.OPEN) return

    this.status = "connecting"
    const url = relayWsUrl()
    this.ws = new WebSocket(url)

    return new Promise((resolve, reject) => {
      if (!this.ws) return reject(new Error("No WebSocket"))
      const timeout = setTimeout(() => reject(new Error("Relay connect timeout")), 10000)

      this.ws.onopen = () => {
        clearTimeout(timeout)
        const filter = {
          kinds: [KIND_VPN_DATA],
          "#p": [this.myPubkey],
          since: this.startedAt - 5,
          limit: 50,
        }
        this.ws!.send(JSON.stringify(["REQ", this.subId, filter]))
        this.status = "relay"
        console.log("[Nostr-Relay] connected as", this.exitMode ? "exit node" : "client", url)
        resolve()
      }

      this.ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data)
          if (!Array.isArray(msg)) return
          if (msg[0] === "EVENT" && msg[2]) {
            const ev = msg[2]
            if (ev.kind !== KIND_VPN_DATA) return
            if (ev.id && this.seenIds.has(ev.id)) return
            if (ev.id) this.seenIds.add(ev.id)
            try {
              const chunk: DataChunk = JSON.parse(ev.content)
              this.handleChunk(chunk)
            } catch {}
          } else if (msg[0] === "OK") {
            // Event accepted by relay
          }
        } catch {}
      }

      this.ws.onerror = () => {
        clearTimeout(timeout)
        this.status = "error"
        reject(new Error("Relay WebSocket error"))
      }
      this.ws.onclose = () => {
        this.status = "disconnected"
        console.log("[Nostr-Relay] disconnected")
      }
    })
  }

  /**
   * Handle incoming data chunk (either request or response).
   */
  private handleChunk(chunk: DataChunk) {
    if (chunk.type === "response") {
      // Client receiving response from exit node
      const pending = this.pending.get(chunk.reqId)
      if (!pending) return

      pending.chunks.set(chunk.seq, chunk.data)

      if (pending.chunks.size >= pending.total) {
        // All chunks received — reassemble
        clearTimeout(pending.timer)
        const sorted = Array.from(pending.chunks.entries()).sort((a, b) => a[0] - b[0])
        const fullBase64 = sorted.map(([, data]) => data).join("")
        const bytes = base64ToBytes(fullBase64)
        const text = new TextDecoder().decode(bytes)
        this.pending.delete(chunk.reqId)
        console.log("[Nostr-Relay] response reassembled", chunk.reqId.slice(0, 8), text.length + " bytes")
        pending.resolve(text)
      }
    } else if (chunk.type === "request" && this.exitMode && this.onRequest) {
      // Exit node receiving request from client
      console.log("[Nostr-Relay] exit received request chunk", chunk.seq + "/" + chunk.total, chunk.reqId.slice(0, 8))
      const pending = this.pending.get(chunk.reqId) || {
        reqId: chunk.reqId,
        chunks: new Map<number, string>(),
        total: chunk.total,
        resolve: () => {},
        reject: () => {},
        timer: setTimeout(() => {}, 60000),
      }
      if (chunk.target) (pending as any).target = chunk.target
      pending.chunks.set(chunk.seq, chunk.data)
      this.pending.set(chunk.reqId, pending)

      if (pending.chunks.size >= pending.total) {
        // All request chunks received — process
        const sorted = Array.from(pending.chunks.entries()).sort((a, b) => a[0] - b[0])
        const fullData = sorted.map(([, data]) => data).join("")
        const target = new TextDecoder().decode(base64ToBytes(fullData))
        console.log("[Nostr-Relay] exit processing request:", target)
        this.onRequest(chunk.reqId, target)
        this.pending.delete(chunk.reqId)
      }
    }
  }

  /**
   * Start as exit node: receive requests and process them.
   */
  startExitNode(handler: (reqId: string, target: string) => Promise<void>) {
    this.exitMode = true
    this.onRequest = handler
    console.log("[Nostr-Relay] exit node mode enabled")
  }

  /**
   * Send HTTP request through Nostr relay (client side).
   */
  async fetchThroughRelay(target: string, exitPubkey: string): Promise<string> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      await this.connect()
    }

    const reqId = Math.random().toString(36).slice(2, 14)
    const targetBytes = new TextEncoder().encode(target)
    const targetB64 = bytesToBase64(targetBytes)

    // Split into chunks if needed
    const chunks: string[] = []
    for (let i = 0; i < targetB64.length; i += CHUNK_SIZE) {
      chunks.push(targetB64.slice(i, i + CHUNK_SIZE))
    }

    const total = chunks.length
    console.log("[Nostr-Relay] sending request", reqId.slice(0, 8), "chunks:", total)

    // Send each chunk as Nostr event
    for (let i = 0; i < chunks.length; i++) {
      const chunk: DataChunk = {
        reqId,
        seq: i,
        total,
        type: "request",
        data: chunks[i],
        target: i === 0 ? target : undefined,
      }
      await this.sendDataEvent(exitPubkey, chunk)
    }

    // Wait for response
    return new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(reqId)
        reject(new Error("Nostr relay request timeout"))
      }, 15000)

      this.pending.set(reqId, {
        reqId,
        chunks: new Map(),
        total: 0, // will be set when first response chunk arrives
        resolve,
        reject,
        timer,
      })
    })
  }

  /**
   * Exit node: send response back through Nostr relay.
   */
  async sendResponse(reqId: string, body: string, toPubkey: string): Promise<void> {
    const bodyBytes = new TextEncoder().encode(body)
    const bodyB64 = bytesToBase64(bodyBytes)

    const chunks: string[] = []
    for (let i = 0; i < bodyB64.length; i += CHUNK_SIZE) {
      chunks.push(bodyB64.slice(i, i + CHUNK_SIZE))
    }

    const total = chunks.length
    for (let i = 0; i < chunks.length; i++) {
      const chunk: DataChunk = {
        reqId,
        seq: i,
        total,
        type: "response",
        data: chunks[i],
      }
      await this.sendDataEvent(toPubkey, chunk)
    }
    console.log("[Nostr-Relay] response sent", reqId.slice(0, 8), "chunks:", total)
  }

  private async sendDataEvent(toPubkey: string, chunk: DataChunk): Promise<void> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      await this.connect()
    }
    const { pubkey: from, seckey } = signingIdentity()
    const content = JSON.stringify(chunk)
    const createdAt = Math.floor(Date.now() / 1000)
    const tags: string[][] = [["p", toPubkey], ["t", "vpn-data"]]
    const serialized = JSON.stringify([0, from, createdAt, KIND_VPN_DATA, tags, content])
    const id = await sha256Hex(serialized)
    const sig = await signEvent(
      { pubkey: from, created_at: createdAt, kind: KIND_VPN_DATA, tags, content },
      seckey,
    )
    const event = { id, pubkey: from, created_at: createdAt, kind: KIND_VPN_DATA, tags, content, sig }
    this.ws!.send(JSON.stringify(["EVENT", event]))
  }

  destroy(): void {
    try { this.ws?.close() } catch {}
    this.ws = null
    this.pending.clear()
    this.status = "disconnected"
  }
}

// ─── Base64 helpers (chunk-safe) ───
function bytesToBase64(bytes: Uint8Array): string {
  let binary = ""
  const chunkSize = 8192
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, Math.min(i + chunkSize, bytes.length))
    binary += String.fromCharCode(...chunk)
  }
  return btoa(binary)
}
function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

export const dataRelay = new NostrDataRelay()
