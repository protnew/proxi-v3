/**
 * WebRTC VPN — P2P DataChannel tunnel browser<->browser.
 *
 * Architecture tables (источник истины):
 *   01_VPN_Primary_Transport: WebRTC DataChannels (Score 178/220)
 *   04_Hole_Punching: STUN + public TURN (Score 154)
 *   26_Signaling: Nostr NIP-44 for SDP/ICE exchange (Score 193)
 *   57_Global_P2P: libp2p circuit relay (Phase 2 Desktop, Score 191)
 *
 * НАШИХ СЕРВЕРОВ НЕТ. Всё работает на бесплатных публичных сервисах:
 *   - STUN: Google, Cloudflare, Nextcloud (10 серверов) — пробивает 85% NAT
 *   - TURN: OpenRelay, ExpressTurn (публичные бесплатные) — fallback 15%
 *   - Signaling: Nostr relays (тысячи публичных, децентрализованные)
 */

export type RTCStatus = "disconnected" | "connecting" | "connected" | "error"

// Архитектурная таблица 58_TURN_Fallback: STUN only для Phase 1.
// STUN не видит трафик — это бесплатная справочная (RFC 5389).
// 4 проверенных провайдера: Google + Cloudflare. Пробивает 85% NAT.
//
// Для оставшихся 15% (CGNAT, симметричный NAT):
//   - Nostr relay data transport (Phase 1.5, таблица 58 winner)
//   - libp2p circuit relay (Phase 2 Desktop, таблица 57 winner)
// Никаких сторонних TURN с неизвестными credentials.
const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
  { urls: "stun:stun1.l.google.com:19302" },
  { urls: "stun:stun2.l.google.com:19302" },
  { urls: "stun:stun.cloudflare.com:3478" },
]

const TYPE_HTTP_GET = 0x01
const TYPE_DATA = 0x02
const TYPE_HTTPS = 0x03
const TYPE_OK = 0x81
const TYPE_ERR = 0x82

function u32(n: number): Uint8Array {
  const b = new Uint8Array(4)
  new DataView(b.buffer).setUint32(0, n, false)
  return b
}
function readU32(b: Uint8Array): number {
  return new DataView(b.buffer, b.byteOffset, 4).getUint32(0, false)
}

interface PendingTunnel {
  resolve: (body: string) => void
  reject: (e: Error) => void
}

async function getIceServers(): Promise<RTCIceServer[]> {
  return ICE_SERVERS
}

export class WebRTCVPNClient {
  private pc: RTCPeerConnection | null = null
  private dc: RTCDataChannel | null = null
  private status: RTCStatus = "disconnected"
  private lastError = ""
  private pending: PendingTunnel | null = null
  private iceRole = ""

  getStatus(): RTCStatus { return this.status }
  getLastError(): string { return this.lastError }
  getIceRole(): string { return this.iceRole }
  isSupported(): boolean { return typeof RTCPeerConnection !== "undefined" }

  async createOffer(): Promise<{ sdp: string; gatherIce: (cb: (c: string) => void) => Promise<void> }> {
    if (!this.isSupported()) throw new Error("WebRTC not supported")
    const ice = await getIceServers()
    this.pc = new RTCPeerConnection({ iceServers: ice, iceTransportPolicy: "all" })
    this.dc = this.pc.createDataChannel("vpn-tunnel", { ordered: true })
    this.setupCallerDC(this.dc)
    const offer = await this.pc.createOffer()
    await this.pc.setLocalDescription(offer)
    this.status = "connecting"
    this.iceRole = "caller"
    const gatherIce = async (cb: (c: string) => void) => {
      if (!this.pc) return
      this.pc.onicecandidate = (e) => { if (e.candidate) cb(e.candidate.candidate) }
      this.pc.onicegatheringstatechange = () => { if (this.pc?.iceGatheringState === "complete") cb("END") }
    }
    return { sdp: offer.sdp || "", gatherIce }
  }

  async applyAnswer(sdpAnswer: string, iceCandidates: string[]): Promise<void> {
    if (!this.pc) throw new Error("No PeerConnection")
    await this.pc.setRemoteDescription({ type: "answer", sdp: sdpAnswer })
    for (const c of iceCandidates) { if (c && c !== "END") { try { await this.pc!.addIceCandidate({ candidate: c, sdpMid: "0", sdpMLineIndex: 0 }) } catch {} } }
  }

  async createAnswer(sdpOffer: string, iceCandidates: string[]): Promise<{ sdp: string; iceCandidates: string[] }> {
    if (!this.isSupported()) throw new Error("WebRTC not supported")
    const ice = await getIceServers()
    this.pc = new RTCPeerConnection({ iceServers: ice, iceTransportPolicy: "all" })
    this.pc.ondatachannel = (e) => { this.dc = e.channel; this.setupExitDC(this.dc) }
    await this.pc.setRemoteDescription({ type: "offer", sdp: sdpOffer })
    for (const c of iceCandidates) { if (c && c !== "END") { try { await this.pc!.addIceCandidate({ candidate: c, sdpMid: "0", sdpMLineIndex: 0 }) } catch {} } }
    const answer = await this.pc.createAnswer()
    await this.pc.setLocalDescription(answer)
    this.status = "connecting"
    this.iceRole = "callee"
    const myCandidates: string[] = []
    await new Promise<void>((resolve) => {
      if (!this.pc) return resolve()
      let done = false
      const finish = () => { if (!done) { done = true; resolve() } }
      this.pc.onicecandidate = (e) => { if (e.candidate) myCandidates.push(e.candidate.candidate); else finish() }
      this.pc.onicegatheringstatechange = () => { if (this.pc?.iceGatheringState === "complete") finish() }
      setTimeout(finish, 5000)
    })
    return { sdp: answer.sdp || "", iceCandidates: myCandidates }
  }

  async waitForOpen(timeoutMs = 15000): Promise<void> {
    if (this.dc?.readyState === "open") { this.status = "connected"; return }
    if (!this.dc) throw new Error("No DataChannel")
    await new Promise<void>((resolve, reject) => {
      const t = setTimeout(() => reject(new Error("DataChannel open timeout")), timeoutMs)
      const interval = setInterval(() => {
        if (this.dc?.readyState === "open") { clearInterval(interval); clearTimeout(t); this.status = "connected"; resolve() }
      }, 200)
      this.dc.onopen = () => { clearInterval(interval); clearTimeout(t); this.status = "connected"; resolve() }
    })
  }

  private setupCallerDC(dc: RTCDataChannel) {
    dc.binaryType = "arraybuffer"
    dc.onmessage = (e) => this.handleCallerMsg(new Uint8Array(e.data))
    dc.onclose = () => { this.status = "disconnected" }
    dc.onerror = (e) => { this.status = "error"; this.lastError = String(e) }
  }

  private setupExitDC(dc: RTCDataChannel) {
    dc.binaryType = "arraybuffer"
    dc.onopen = () => { console.log("[WebRTC-VPN] Exit DataChannel open"); this.status = "connected" }
    dc.onmessage = async (e) => {
      const data = new Uint8Array(e.data)
      if (data.length < 5) return
      const type = data[0]
      const len = readU32(data.slice(1, 5))
      const payload = data.slice(5, 5 + len)
      if (type === TYPE_HTTP_GET) {
        const target = new TextDecoder().decode(payload)
        try {
          const url = target.startsWith("http") ? target : "http://" + target
          const resp = await fetch(url)
          const body = new Uint8Array(await resp.arrayBuffer())
          this.sendReply(dc, TYPE_OK, body)
        } catch (err) { this.sendReply(dc, TYPE_ERR, new TextEncoder().encode((err as Error).message)) }
      } else if (type === TYPE_HTTPS) {
        const target = new TextDecoder().decode(payload)
        try {
          const url = target.startsWith("https") ? target : "https://" + target
          const resp = await fetch(url)
          const body = new Uint8Array(await resp.arrayBuffer())
          this.sendReply(dc, TYPE_OK, body)
        } catch (err) { this.sendReply(dc, TYPE_ERR, new TextEncoder().encode((err as Error).message)) }
      }
    }
    dc.onclose = () => { this.status = "disconnected" }
  }

  private sendReply(dc: RTCDataChannel, type: number, body: Uint8Array) {
    const hdr = new Uint8Array(5); hdr[0] = type; hdr.set(u32(body.length), 1)
    const merged = new Uint8Array(5 + body.length); merged.set(hdr, 0); merged.set(body, 5)
    if (dc.readyState === "open") dc.send(merged)
  }

  private handleCallerMsg(data: Uint8Array) {
    if (data.length < 5) return
    const type = data[0]
    const len = readU32(data.slice(1, 5))
    const payload = data.slice(5, 5 + len)
    if (!this.pending) return
    if (type === TYPE_OK) { const p = this.pending; this.pending = null; p.resolve(new TextDecoder().decode(payload)) }
    else if (type === TYPE_ERR) { const p = this.pending; this.pending = null; p.reject(new Error(new TextDecoder().decode(payload))) }
  }

  async fetchHTTP(urlStr: string): Promise<string> {
    if (!this.dc || this.dc.readyState !== "open") throw new Error("DataChannel not open")
    const payload = new TextEncoder().encode(urlStr)
    const msg = new Uint8Array(5 + payload.length); msg[0] = TYPE_HTTP_GET; msg.set(u32(payload.length), 1); msg.set(payload, 5)
    return this.sendAndWait(msg)
  }

  async fetchHTTPS(urlStr: string): Promise<string> {
    if (!this.dc || this.dc.readyState !== "open") throw new Error("DataChannel not open")
    const payload = new TextEncoder().encode(urlStr)
    const msg = new Uint8Array(5 + payload.length); msg[0] = TYPE_HTTPS; msg.set(u32(payload.length), 1); msg.set(payload, 5)
    return this.sendAndWait(msg)
  }

  async fetchThroughTunnel(urlStr: string): Promise<string> {
    if (urlStr.startsWith("https://")) return this.fetchHTTPS(urlStr)
    return this.fetchHTTP(urlStr)
  }

  private sendAndWait(msg: Uint8Array): Promise<string> {
    return new Promise<string>((resolve, reject) => {
      this.pending = { resolve, reject }
      this.dc!.send(msg)
      setTimeout(() => { if (this.pending) { this.pending = null; reject(new Error("Tunnel fetch timeout")) } }, 10000)
    })
  }

  async disconnect(): Promise<void> {
    try { this.dc?.close() } catch {}
    try { this.pc?.close() } catch {}
    this.dc = null; this.pc = null; this.status = "disconnected"
  }
}

export const rtcVPN = new WebRTCVPNClient()
