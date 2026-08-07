/**
 * WebRTC VPN - P2P DataChannel tunnel browser<->browser.
 * Architecture table 01_VPN_Primary_Transport: WebRTC DataChannels (Score 178/220).
 * Replaces WebTransport (server-client) with true P2P + NAT traversal (ICE/STUN/TURN).
 */

export type RTCStatus = "disconnected" | "connecting" | "connected" | "error"

/** STUN servers from architecture table 04_Hole_Punching (free, public). */
const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
  { urls: "stun:stun1.l.google.com:19302" },
  { urls: "stun:stun.cloudflare.com:3478" },
]

const TYPE_CONNECT = 0x01
const TYPE_DATA = 0x02
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

/**
 * WebRTC VPN - caller side (wants VPN from friend).
 * Initiates RTCPeerConnection, creates DataChannel, tunnels HTTP through it.
 */
export class WebRTCVPNClient {
  private pc: RTCPeerConnection | null = null
  private dc: RTCDataChannel | null = null
  private status: RTCStatus = "disconnected"
  private lastError = ""
  private pending: PendingTunnel | null = null

  getStatus(): RTCStatus { return this.status }
  getLastError(): string { return this.lastError }
  isSupported(): boolean { return typeof RTCPeerConnection !== "undefined" }

  /** Create SDP offer (caller side). Returns offer + ICE gathering callback. */
  async createOffer(): Promise<{ sdp: string; gatherIce: (cb: (c: string) => void) => Promise<void> }> {
    if (!this.isSupported()) throw new Error("WebRTC not supported")
    this.pc = new RTCPeerConnection({ iceServers: ICE_SERVERS })
    this.dc = this.pc.createDataChannel("vpn-tunnel", { ordered: true })
    this.setupCallerDC(this.dc)

    const offer = await this.pc.createOffer()
    await this.pc.setLocalDescription(offer)
    this.status = "connecting"

    const gatherIce = async (cb: (c: string) => void) => {
      if (!this.pc) return
      this.pc.onicecandidate = (e) => { if (e.candidate) cb(e.candidate.candidate) }
      this.pc.onicegatheringstatechange = () => {
        if (this.pc?.iceGatheringState === "complete") cb("END")
      }
    }
    return { sdp: offer.sdp || "", gatherIce }
  }

  /** Apply SDP answer from callee. */
  async applyAnswer(sdpAnswer: string, iceCandidates: string[]): Promise<void> {
    if (!this.pc) throw new Error("No PeerConnection")
    await this.pc.setRemoteDescription({ type: "answer", sdp: sdpAnswer })
    for (const c of iceCandidates) {
      if (c && c !== "END") {
        try { await this.pc!.addIceCandidate({ candidate: c, sdpMid: "0", sdpMLineIndex: 0 }) } catch {}
      }
    }
  }

  /** Create SDP answer (callee/exit-node side). */
  async createAnswer(sdpOffer: string, iceCandidates: string[]): Promise<{ sdp: string; iceCandidates: string[] }> {
    if (!this.isSupported()) throw new Error("WebRTC not supported")
    this.pc = new RTCPeerConnection({ iceServers: ICE_SERVERS })
    this.pc.ondatachannel = (e) => { this.dc = e.channel; this.setupExitDC(this.dc) }

    await this.pc.setRemoteDescription({ type: "offer", sdp: sdpOffer })
    for (const c of iceCandidates) {
      if (c && c !== "END") {
        try { await this.pc!.addIceCandidate({ candidate: c, sdpMid: "0", sdpMLineIndex: 0 }) } catch {}
      }
    }
    const answer = await this.pc.createAnswer()
    await this.pc.setLocalDescription(answer)
    this.status = "connecting"

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

  /** Wait for DataChannel open. */
  async waitForOpen(timeoutMs = 15000): Promise<void> {
    if (this.dc?.readyState === "open") { this.status = "connected"; return }
    if (!this.dc) throw new Error("No DataChannel")
    await new Promise<void>((resolve, reject) => {
      const t = setTimeout(() => reject(new Error("DataChannel open timeout")), timeoutMs)
      const interval = setInterval(() => {
        if (this.dc?.readyState === "open") {
          clearInterval(interval); clearTimeout(t); this.status = "connected"; resolve()
        }
      }, 200)
      this.dc.onopen = () => {
        clearInterval(interval); clearTimeout(t); this.status = "connected"; resolve()
      }
    })
  }

  private setupCallerDC(dc: RTCDataChannel) {
    dc.binaryType = "arraybuffer"
    dc.onmessage = (e) => this.handleCallerMsg(new Uint8Array(e.data))
    dc.onclose = () => { this.status = "disconnected" }
    dc.onerror = (e) => { this.status = "error"; this.lastError = String(e) }
  }

  /** Exit node: receives CONNECT, does fetch, returns body. */
  private setupExitDC(dc: RTCDataChannel) {
    dc.binaryType = "arraybuffer"
    dc.onopen = () => { console.log("[WebRTC-VPN] Exit DataChannel open"); this.status = "connected" }
    dc.onmessage = async (e) => {
      const data = new Uint8Array(e.data)
      if (data.length < 5) return
      const type = data[0]
      const len = readU32(data.slice(1, 5))
      const payload = data.slice(5, 5 + len)
      if (type === TYPE_CONNECT) {
        const target = new TextDecoder().decode(payload)
        console.log("[WebRTC-VPN] Exit CONNECT", target)
        try {
          const resp = await fetch("http://" + target)
          const body = new Uint8Array(await resp.arrayBuffer())
          const hdr = new Uint8Array(5)
          hdr[0] = TYPE_OK; hdr.set(u32(body.length), 1)
          const merged = new Uint8Array(5 + body.length)
          merged.set(hdr, 0); merged.set(body, 5)
          if (dc.readyState === "open") dc.send(merged)
        } catch (err) {
          const msg = new TextEncoder().encode((err as Error).message)
          const hdr = new Uint8Array(5)
          hdr[0] = TYPE_ERR; hdr.set(u32(msg.length), 1)
          const merged = new Uint8Array(5 + msg.length)
          merged.set(hdr, 0); merged.set(msg, 5)
          if (dc.readyState === "open") dc.send(merged)
        }
      }
    }
    dc.onclose = () => { this.status = "disconnected" }
  }

  private handleCallerMsg(data: Uint8Array) {
    if (data.length < 5) return
    const type = data[0]
    const len = readU32(data.slice(1, 5))
    const payload = data.slice(5, 5 + len)
    if (!this.pending) return
    if (type === TYPE_OK) {
      const p = this.pending; this.pending = null
      p.resolve(new TextDecoder().decode(payload))
    } else if (type === TYPE_ERR) {
      const p = this.pending; this.pending = null
      p.reject(new Error(new TextDecoder().decode(payload)))
    }
  }

  /** Client: fetch URL through P2P DataChannel tunnel. */
  async fetchThroughTunnel(urlStr: string): Promise<string> {
    if (!this.dc || this.dc.readyState !== "open") throw new Error("DataChannel not open")
    const u = new URL(urlStr)
    const port = u.port || (u.protocol === "https:" ? "443" : "80")
    const target = u.hostname + ":" + port
    const payload = new TextEncoder().encode(target)
    const msg = new Uint8Array(5 + payload.length)
    msg[0] = TYPE_CONNECT; msg.set(u32(payload.length), 1); msg.set(payload, 5)

    return new Promise<string>((resolve, reject) => {
      this.pending = { resolve, reject }
      this.dc!.send(msg)
      setTimeout(() => {
        if (this.pending) { this.pending = null; reject(new Error("Tunnel fetch timeout")) }
      }, 10000)
    })
  }

  async disconnect(): Promise<void> {
    try { this.dc?.close() } catch {}
    try { this.pc?.close() } catch {}
    this.dc = null; this.pc = null; this.status = "disconnected"
  }
}

export const rtcVPN = new WebRTCVPNClient()
