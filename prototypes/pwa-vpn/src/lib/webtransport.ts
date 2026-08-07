/**
 * WebTransport VPN client — connects to friend's exit node via QUIC.
 * Architecture table T2: WebTransport (QUIC) is the chosen PWA VPN transport.
 *
 * Flow:
 * 1. Receive WT address + cert hash via Nostr signaling (kind:30090 vpn-accept)
 * 2. Connect via WebTransport API
 * 3. Open bi-directional streams for proxied traffic
 */

export type WTConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface WTStats {
  state: WTConnectionState
  addr: string
  bytesIn: number
  bytesOut: number
  streams: number
}

// Message types matching Go server wire protocol
const MSG_CONNECT = 0x01
const MSG_CONNECT_OK = 0x81
const MSG_ERROR = 0xFF

export class WebTransportVPN {
  private transport: WebTransport | null = null
  private state: WTConnectionState = 'disconnected'
  private bytesIn = 0
  private bytesOut = 0
  private activeStreams = 0
  private listeners: ((state: WTConnectionState) => void)[] = []

  /**
   * Connect to a friend's WebTransport exit node.
   * @param addr — host:port of the exit node's WT server
   * @param certHash — SHA-256 hash of the self-signed certificate (for verification)
   */
  async connect(addr: string, certHash?: string): Promise<void> {
    if (typeof WebTransport === 'undefined') {
      throw new Error('WebTransport API not available in this browser')
    }

    this.setState('connecting')

    try {
      // WebTransport with serverCertificateHashes (Chrome 115+)
      const options: WebTransportOptions = {}
      if (certHash) {
        // Pass cert hash for self-signed cert verification
        options.serverCertificateHashes = [{
          algorithm: 'sha-256',
          value: hexToUint8Array(certHash),
        }]
      }

      const url = `https://${addr}/wt`
      this.transport = new WebTransport(url, options)

      await this.transport.ready
      this.setState('connected')
      console.log('[WT-VPN] Connected to exit node:', addr)
    } catch (err) {
      this.setState('error')
      throw new Error(`WebTransport connect failed: ${err}`)
    }
  }

  /**
   * Open a tunnel to a target host:port through the exit node.
   * Returns a readable/writable stream pair for bidirectional data.
   */
  async openTunnel(target: string): Promise<{ readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> }> {
    if (!this.transport || this.state !== 'connected') {
      throw new Error('Not connected to exit node')
    }

    const stream = await this.transport.createBidirectionalStream()
    const writer = stream.writable.getWriter()

    // Send CONNECT frame: [type][length][target]
    const targetBytes = new TextEncoder().encode(target)
    const header = new ArrayBuffer(5)
    const view = new DataView(header)
    view.setUint8(0, MSG_CONNECT)
    view.setUint32(1, targetBytes.length, false) // big-endian

    await writer.write(header)
    await writer.write(targetBytes)
    this.bytesOut += 5 + targetBytes.length

    // Read response header
    const reader = stream.readable.getReader()
    const { value: respHeader } = await reader.read()
    if (!respHeader || respHeader[0] !== MSG_CONNECT_OK) {
      const errorLen = respHeader ? new DataView(respHeader.buffer).getUint32(1, false) : 0
      let errorMsg = 'unknown error'
      if (errorLen > 0 && respHeader) {
        const { value: errBody } = await reader.read()
        if (errBody) errorMsg = new TextDecoder().decode(errBody)
      }
      throw new Error(`Tunnel connect failed: ${errorMsg}`)
    }

    this.activeStreams++
    console.log('[WT-VPN] Tunnel established to', target)

    return {
      readable: stream.readable,
      writable: stream.writable,
    }
  }

  /**
   * Disconnect from the exit node.
   */
  async disconnect(): Promise<void> {
    if (this.transport) {
      await this.transport.close()
      this.transport = null
    }
    this.activeStreams = 0
    this.setState('disconnected')
    console.log('[WT-VPN] Disconnected')
  }

  /**
   * Get current connection stats.
   */
  getStats(): WTStats {
    return {
      state: this.state,
      addr: this.transport?.constructor?.name === 'WebTransport' ? 'connected' : '',
      bytesIn: this.bytesIn,
      bytesOut: this.bytesOut,
      streams: this.activeStreams,
    }
  }

  /**
   * Subscribe to state changes.
   */
  onStateChange(callback: (state: WTConnectionState) => void): () => void {
    this.listeners.push(callback)
    return () => {
      this.listeners = this.listeners.filter(l => l !== callback)
    }
  }

  private setState(s: WTConnectionState) {
    this.state = s
    this.listeners.forEach(cb => cb(s))
  }
}

// Helper: hex string to Uint8Array
function hexToUint8Array(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  }
  return bytes
}

// Singleton instance
export const wtVPN = new WebTransportVPN()
