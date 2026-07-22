/**
 * Chunked file transfer over WebRTC DataChannel
 * Protocol:
 *   1. Sender: create manifest (name, size, chunks, hash)
 *   2. Sender: send manifest via Nostr
 *   3. Receiver: create peer connection (or reuse existing)
 *   4. Sender: send chunks sequentially
 *   5. Receiver: assemble and verify
 */

export interface FileManifest {
  id: string
  name: string
  size: number
  mimeType: string
  chunks: number
  chunkSize: number
  hash: string      // SHA-256 of entire file
  from: string      // sender pubkey
}

export interface FileChunk {
  manifestId: string
  index: number
  data: ArrayBuffer
}

const CHUNK_SIZE = 64 * 1024 // 64KB per chunk (DataChannel safe)

/**
 * Split file into chunks, compute hash
 */
export async function createManifest(file: File, senderPubkey: string): Promise<{
  manifest: FileManifest
  chunks: ArrayBuffer[]
}> {
  const buffer = await file.arrayBuffer()
  const hash = await computeHash(buffer)
  
  const totalChunks = Math.ceil(buffer.byteLength / CHUNK_SIZE)
  const chunks: ArrayBuffer[] = []
  
  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, buffer.byteLength)
    chunks.push(buffer.slice(start, end))
  }

  return {
    manifest: {
      id: crypto.randomUUID(),
      name: file.name,
      size: file.size,
      mimeType: file.type || 'application/octet-stream',
      chunks: totalChunks,
      chunkSize: CHUNK_SIZE,
      hash,
      from: senderPubkey,
    },
    chunks,
  }
}

/**
 * Reassemble chunks into a Blob
 */
export async function reassemble(
  manifest: FileManifest,
  chunks: Map<number, ArrayBuffer>
): Promise<{ blob: Blob; valid: boolean }> {
  const ordered = new Array<ArrayBuffer>(manifest.chunks)
  for (const [i, chunk] of chunks) {
    ordered[i] = chunk
  }
  
  const blob = new Blob(ordered, { type: manifest.mimeType })
  
  // Verify hash
  const buffer = await blob.arrayBuffer()
  const hash = await computeHash(buffer)
  const valid = hash === manifest.hash
  
  return { blob, valid }
}

/**
 * Send file chunks over DataChannel with flow control
 */
export async function sendChunks(
  dc: RTCDataChannel,
  manifest: FileManifest,
  chunks: ArrayBuffer[],
  onProgress?: (sent: number, total: number) => void
): Promise<void> {
  // Wait for channel to be open
  if (dc.readyState !== 'open') {
    await new Promise<void>(resolve => {
      dc.onopen = () => resolve()
      setTimeout(resolve, 10000)
    })
  }

  // Send manifest first
  dc.send(JSON.stringify({ type: 'file-manifest', manifest }))
  
  // Send chunks with buffering
  for (let i = 0; i < chunks.length; i++) {
    // Wait if buffer is full
    while (dc.bufferedAmount > 1024 * 1024) { // 1MB buffer limit
      await new Promise(r => setTimeout(r, 50))
    }
    
    // Send chunk header + data
    const header = JSON.stringify({ type: 'file-chunk', manifestId: manifest.id, index: i })
    dc.send(header)
    dc.send(chunks[i])
    
    onProgress?.(i + 1, chunks.length)
  }
  
  // Send completion signal
  dc.send(JSON.stringify({ type: 'file-complete', manifestId: manifest.id }))
}

/**
 * Receive file chunks from DataChannel messages
 */
export class FileReceiver {
  private manifests = new Map<string, FileManifest>()
  private chunks = new Map<string, Map<number, ArrayBuffer>>()
  private callbacks = new Map<string, {
    onProgress?: (received: number, total: number) => void
    onComplete?: (blob: Blob, manifest: FileManifest) => void
    onError?: (error: string) => void
  }>()

  register(manifestId: string, callbacks: {
    onProgress?: (received: number, total: number) => void
    onComplete?: (blob: Blob, manifest: FileManifest) => void
    onError?: (error: string) => void
  }) {
    this.callbacks.set(manifestId, callbacks)
  }

  handleMessage(data: any) {
    if (typeof data === 'string') {
      try {
        const msg = JSON.parse(data)
        if (msg.type === 'file-manifest') {
          this.manifests.set(msg.manifest.id, msg.manifest)
          this.chunks.set(msg.manifest.id, new Map())
        }
        if (msg.type === 'file-complete') {
          this.complete(msg.manifestId)
        }
      } catch {}
    } else if (data instanceof ArrayBuffer) {
      // This would be handled in conjunction with the previous header message
      // In practice, we'd track the expected next chunk
    }
  }

  handleChunk(manifestId: string, index: number, buffer: ArrayBuffer) {
    let chunkMap = this.chunks.get(manifestId)
    if (!chunkMap) {
      chunkMap = new Map()
      this.chunks.set(manifestId, chunkMap)
    }
    chunkMap.set(index, buffer)
    
    const manifest = this.manifests.get(manifestId)
    const cbs = this.callbacks.get(manifestId)
    if (manifest && cbs) {
      cbs.onProgress?.(chunkMap.size, manifest.chunks)
    }
  }

  private async complete(manifestId: string) {
    const manifest = this.manifests.get(manifestId)
    const chunkMap = this.chunks.get(manifestId)
    const cbs = this.callbacks.get(manifestId)
    
    if (!manifest || !chunkMap) return
    
    const { blob, valid } = await reassemble(manifest, chunkMap)
    
    if (!valid) {
      cbs?.onError?.('Hash mismatch')
      return
    }
    
    cbs?.onComplete?.(blob, manifest)
    
    // Cleanup
    this.manifests.delete(manifestId)
    this.chunks.delete(manifestId)
    this.callbacks.delete(manifestId)
  }
}

async function computeHash(buffer: ArrayBuffer): Promise<string> {
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
  return Array.from(new Uint8Array(hashBuffer)).map(b => b.toString(16).padStart(2, '0')).join('')
}
