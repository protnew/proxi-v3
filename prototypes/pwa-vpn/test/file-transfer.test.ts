/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest';
import { createManifest, reassemble, FileReceiver } from '../src/lib/file-transfer';

describe('file-transfer', () => {
  it('createManifest generates manifest for a file', async () => {
    const file = new File(['test file content'], 'test.txt', { type: 'text/plain' });
    const result = await createManifest(file, 'sender_pubkey_123');
    expect(result).toBeDefined();
    expect(result.manifest.name).toBe('test.txt');
    expect(result.manifest.chunks).toBeGreaterThan(0);
    expect(result.manifest.size).toBe(file.size);
    expect(result.manifest.from).toBe('sender_pubkey_123');
    expect(result.chunks.length).toBeGreaterThan(0);
  });

  it('createManifest with large file produces multiple chunks', async () => {
    const data = new Uint8Array(64 * 1024 + 100); // slightly over CHUNK_SIZE
    const file = new File([data], 'big.bin', { type: 'application/octet-stream' });
    const result = await createManifest(file, 'sender_pubkey');
    expect(result.manifest.chunks).toBe(2);
  });

  it('createManifest computes hash', async () => {
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' });
    const result = await createManifest(file, 'sender');
    expect(result.manifest.hash).toMatch(/^[0-9a-f]{64}$/);
  });

  it('reassemble combines chunks back into blob', async () => {
    const file = new File(['Hello World'], 'test.txt', { type: 'text/plain' });
    const { manifest, chunks } = await createManifest(file, 'sender');
    const chunkMap = new Map<number, ArrayBuffer>();
    chunks.forEach((chunk, i) => chunkMap.set(i, chunk));
    const result = await reassemble(manifest, chunkMap);
    expect(result.blob).toBeDefined();
    expect(result.valid).toBe(true);
  });

  it('FileReceiver can be instantiated', () => {
    const receiver = new FileReceiver();
    expect(receiver).toBeDefined();
  });
});
