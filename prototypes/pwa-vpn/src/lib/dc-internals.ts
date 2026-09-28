/**
 * INF-004: DataChannel internals
 * - T06: Stream multiplexing (multiple streams over single DC)
 * - T07: Packet serialization (binary framing)
 * - T08: Heartbeat (keepalive)
 */

export enum StreamType {
  HTTP = 0x01,
  SOCKS5 = 0x02,
  DNS = 0x03,
  RAW = 0x04,
  HEARTBEAT = 0x05,
}

export interface Packet {
  streamId: number
  type: StreamType
  flags: number
  payload: Uint8Array
}

// T07: Serialize packet to binary (4-byte header + payload)
// | streamId (2B) | type (1B) | flags (1B) | payload (variable) |
export function serializePacket(pkt: Packet): Uint8Array {
  const buf = new Uint8Array(4 + pkt.payload.length)
  const view = new DataView(buf.buffer)
  view.setUint16(0, pkt.streamId)
  view.setUint8(2, pkt.type)
  view.setUint8(3, pkt.flags)
  buf.set(pkt.payload, 4)
  return buf
}

export function deserializePacket(data: ArrayBuffer | Uint8Array): Packet {
  const buf = data instanceof Uint8Array ? data : new Uint8Array(data)
  if (buf.length < 4) throw new Error('Packet too short')
  const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength)
  return {
    streamId: view.getUint16(0),
    type: view.getUint8(2) as StreamType,
    flags: view.getUint8(3),
    payload: buf.slice(4),
  }
}

// T06: Stream multiplexer
export class StreamMultiplexer {
  private streams = new Map<number, { onData: (data: Uint8Array) => void; onClose: () => void }>()
  private nextId = 1
  private dc: RTCDataChannel | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null

  attach(dc: RTCDataChannel) {
    this.dc = dc
    dc.onmessage = (e) => {
      try {
        const pkt = deserializePacket(e.data)
        if (pkt.type === StreamType.HEARTBEAT) {
          // Reply to heartbeat
          this.send(pkt.streamId, StreamType.HEARTBEAT, new Uint8Array([1]))
          return
        }
        const stream = this.streams.get(pkt.streamId)
        if (stream) stream.onData(pkt.payload)
      } catch {}
    }
    dc.onclose = () => {
      this.streams.forEach(s => s.onClose())
      this.streams.clear()
      if (this.heartbeatTimer) clearInterval(this.heartbeatTimer)
    }
    this.startHeartbeat()
  }

  // T08: Heartbeat every 15s
  private startHeartbeat() {
    this.heartbeatTimer = setInterval(() => {
      if (this.dc?.readyState === 'open') {
        this.send(0, StreamType.HEARTBEAT, new Uint8Array([0]))
      }
    }, 15000)
  }

  createStream(type: StreamType, onData: (data: Uint8Array) => void, onClose?: () => void): number {
    const id = this.nextId++
    this.streams.set(id, { onData, onClose: onClose || (() => {}) })
    return id
  }

  send(streamId: number, type: StreamType, payload: Uint8Array, flags = 0): boolean {
    if (!this.dc || this.dc.readyState !== 'open') return false
    const pkt = serializePacket({ streamId, type, flags, payload })
    this.dc.send(pkt)
    return true
  }

  closeStream(streamId: number) {
    const s = this.streams.get(streamId)
    if (s) {
      s.onClose()
      this.streams.delete(streamId)
    }
  }

  get activeStreams(): number {
    return this.streams.size
  }
}
