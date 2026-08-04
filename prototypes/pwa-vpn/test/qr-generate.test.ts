/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest';
import { generateContactQR, parseQRContent } from '../src/lib/qr';

describe('QR code generation', () => {
  it('generateContactQR returns data URL string', async () => {
    const result = await generateContactQR({
      type: 'nostr', pubkey: 'a'.repeat(64), name: 'Alice'
    });
    expect(typeof result).toBe('string');
    expect(result.startsWith('data:image')).toBe(true);
  });

  it('generateContactQR includes relays when provided', async () => {
    const result = await generateContactQR({
      type: 'nostr', pubkey: 'b'.repeat(64), name: 'Bob',
      relays: ['wss://relay.damus.io', 'wss://nos.lol']
    });
    expect(typeof result).toBe('string');
    expect(result.startsWith('data:image')).toBe(true);
  });

  it('parseQRContent parses valid JSON nostr contact', () => {
    const json = JSON.stringify({ type: 'nostr', pubkey: 'c'.repeat(64), name: 'Carol' });
    const result = parseQRContent(json);
    expect(result).not.toBeNull();
    expect(result!.type).toBe('nostr');
    expect(result!.pubkey).toBe('c'.repeat(64));
  });

  it('parseQRContent handles nostr: URI', () => {
    const result = parseQRContent('nostr:npub1abc123');
    expect(result).not.toBeNull();
    expect(result!.pubkey).toContain('npub1');
  });

  it('parseQRContent returns null for garbage', () => {
    expect(parseQRContent('not json not nostr')).toBeNull();
    expect(parseQRContent('')).toBeNull();
    expect(parseQRContent('12345')).toBeNull();
  });

  it('parseQRContent returns null for invalid JSON without nostr type', () => {
    expect(parseQRContent(JSON.stringify({ foo: 'bar' }))).toBeNull();
  });
});
