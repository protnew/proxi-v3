/**
 * WebTransport VPN client — real HTTP/3 WT to friend's exit (architecture T2).
 */

export type WTStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.replace(/^0x/, '').trim()
  if (clean.length % 2 !== 0) throw new Error('odd hex length')
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
  }
  return out
}

/** Stream reader that never drops leftover bytes from chunked reads. */
class BufferedReader {
  private reader: ReadableStreamDefaultReader<Uint8Array>
  private buf = new Uint8Array(0)

  constructor(readable: ReadableStream<Uint8Array>) {
    this.reader = readable.getReader()
  }

  async readExact(n: number): Promise<Uint8Array> {
    while (this.buf.byteLength < n) {
      const { value, done } = await this.reader.read()
      if (done) {
        throw new Error(`stream closed reading ${this.buf.byteLength}/${n}`)
      }
      if (!value?.byteLength) continue
      const merged = new Uint8Array(this.buf.byteLength + value.byteLength)
      merged.set(this.buf, 0)
      merged.set(value, this.buf.byteLength)
      this.buf = merged
    }
    const out = this.buf.slice(0, n)
    this.buf = this.buf.slice(n)
    return out
  }

  async readFrame(): Promise<{ type: number; payload: Uint8Array }> {
    const hdr = await this.readExact(5)
    const type = hdr[0]
    const len = new DataView(hdr.buffer, hdr.byteOffset, 5).getUint32(1)
    if (len > 1_000_000) throw new Error('frame too large: ' + len)
    const payload = len > 0 ? await this.readExact(len) : new Uint8Array(0)
    return { type, payload }
  }

  /** Remaining buffered + live stream as a ReadableStream for TCP body. */
  toBodyStream(): ReadableStream<Uint8Array> {
    const leftover = this.buf
    this.buf = new Uint8Array(0)
    const reader = this.reader
    return new ReadableStream<Uint8Array>({
      start(controller) {
        if (leftover.byteLength) controller.enqueue(leftover)
      },
      async pull(controller) {
        const { value, done } = await reader.read()
        if (done) {
          controller.close()
          return
        }
        if (value) controller.enqueue(value)
      },
      cancel() {
        try {
          reader.cancel()
        } catch {
          /* ignore */
        }
      },
    })
  }

  release() {
    try {
      this.reader.releaseLock()
    } catch {
      /* ignore */
    }
  }
}

export class WTVPNClient {
  private session: WebTransport | null = null
  private status: WTStatus = 'disconnected'
  private lastError = ''
  private remoteAddr = ''

  getStatus(): WTStatus {
    return this.status
  }
  getLastError(): string {
    return this.lastError
  }
  getRemoteAddr(): string {
    return this.remoteAddr
  }
  isSupported(): boolean {
    return typeof WebTransport !== 'undefined'
  }

  async connect(addr: string, certHashHex?: string): Promise<void> {
    if (!this.isSupported()) throw new Error('WebTransport not supported in this browser')
    this.status = 'connecting'
    this.lastError = ''
    this.remoteAddr = addr

    const url = addr.startsWith('https://')
      ? addr.includes('/wt')
        ? addr
        : addr.replace(/\/?$/, '/wt')
      : `https://${addr}/wt`

    const opts: WebTransportOptions = { allowPooling: false }
    if (certHashHex && certHashHex.length >= 64) {
      opts.serverCertificateHashes = [
        { algorithm: 'sha-256', value: hexToBytes(certHashHex.slice(0, 64)) },
      ]
    }

    try {
      const transport = new WebTransport(url, opts)
      await transport.ready
      this.session = transport
      this.status = 'connected'
      console.log('[WT-VPN] Connected', addr)
      transport.closed
        .then(() => {
          this.status = 'disconnected'
          this.session = null
        })
        .catch((e) => {
          this.status = 'error'
          this.lastError = String(e)
          this.session = null
        })
    } catch (e) {
      this.status = 'error'
      this.lastError = e instanceof Error ? e.message : String(e)
      this.session = null
      throw e
    }
  }

  async disconnect(): Promise<void> {
    if (this.session) {
      try {
        this.session.close()
      } catch {
        /* ignore */
      }
      this.session = null
    }
    this.status = 'disconnected'
  }

  async openConnect(targetHostPort: string): Promise<{
    readable: ReadableStream<Uint8Array>
    writable: WritableStream<Uint8Array>
  }> {
    if (!this.session || this.status !== 'connected') {
      throw new Error('WebTransport not connected')
    }
    const { readable, writable } = await this.session.createBidirectionalStream()
    const writer = writable.getWriter()
    try {
      const payload = new TextEncoder().encode(targetHostPort)
      const hdr = new Uint8Array(5)
      hdr[0] = 0x01
      new DataView(hdr.buffer).setUint32(1, payload.byteLength)
      await writer.write(hdr)
      await writer.write(payload)
    } finally {
      writer.releaseLock()
    }

    const br = new BufferedReader(readable)
    const resp = await br.readFrame()
    if (resp.type !== 0x81) {
      const msg = new TextDecoder().decode(resp.payload)
      br.release()
      throw new Error(`CONNECT failed: ${msg || '0x' + resp.type.toString(16)}`)
    }
    // Body stream continues after OK frame
    return { readable: br.toBodyStream(), writable }
  }

  async fetchHTTP(urlStr: string): Promise<string> {
    const u = new URL(urlStr)
    const port = u.port || (u.protocol === 'https:' ? '443' : '80')
    if (u.protocol === 'https:') {
      throw new Error('HTTPS via tunnel needs TLS-in-CONNECT (use http:// for probe)')
    }
    const { readable, writable } = await this.openConnect(`${u.hostname}:${port}`)
    const writer = writable.getWriter()
    const reader = readable.getReader()
    try {
      const path = (u.pathname || '/') + (u.search || '')
      const req =
        `GET ${path} HTTP/1.1\r\nHost: ${u.host}\r\nConnection: close\r\nUser-Agent: IndestructibleVPN/1.0\r\n\r\n`
      await writer.write(new TextEncoder().encode(req))
      // Do not close write side immediately — some stacks FIN early and drop response.
      const chunks: Uint8Array[] = []
      let total = 0
      const deadline = Date.now() + 8000
      while (total < 65536 && Date.now() < deadline) {
        const readPromise = reader.read()
        const timed = await Promise.race([
          readPromise.then((r) => ({ r, t: false as const })),
          new Promise<{ r: null; t: true }>((res) => setTimeout(() => res({ r: null, t: true }), 2000)),
        ])
        if (timed.t) {
          if (total > 0) break
          continue
        }
        const { value, done } = timed.r!
        if (done) break
        if (value) {
          chunks.push(value)
          total += value.byteLength
          // stop after headers+body for small responses
          const sofar = new TextDecoder().decode(value)
          if (total > 200 && sofar.includes('</html>')) break
          if (total > 20 && !sofar.includes('HTTP/') && /^[\d.]+$/.test(new TextDecoder().decode(chunks[0] || new Uint8Array()))) break
        }
      }
      try {
        await writer.close()
      } catch {
        /* ignore */
      }
      const buf = new Uint8Array(total)
      let off = 0
      for (const c of chunks) {
        buf.set(c, off)
        off += c.byteLength
      }
      const text = new TextDecoder().decode(buf)
      const idx = text.indexOf('\r\n\r\n')
      return idx >= 0 ? text.slice(idx + 4) : text
    } finally {
      try {
        reader.releaseLock()
      } catch {
        /* ignore */
      }
      try {
        writer.releaseLock()
      } catch {
        /* ignore */
      }
    }
  }
}

export const wtVPN = new WTVPNClient()
