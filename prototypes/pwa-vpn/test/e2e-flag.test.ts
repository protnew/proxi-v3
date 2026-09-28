import { describe, it, expect, beforeEach, vi } from 'vitest';

const store: Record<string, string> = {};
globalThis.localStorage = {
  getItem: (k: string) => store[k] || null,
  setItem: (k: string, v: string) => { store[k] = v; },
  removeItem: (k: string) => { delete store[k]; },
  clear: () => { Object.keys(store).forEach(k => delete store[k]); },
} as any;

globalThis.fetch = vi.fn() as any;

describe('MSG-009 isE2EEnabled on send path (P5)', () => {
  beforeEach(() => {
    Object.keys(store).forEach(k => delete store[k]);
    vi.resetModules();
  });

  it('buildChatPayload.encrypted false for plaintext even when E2E on', async () => {
    const api = await import('../src/lib/api.ts');
    api.setIdentity('a'.repeat(64), 'b'.repeat(64));
    api.setE2EEnabledLocal(true);
    const p = api.buildChatPayload('bobpk', 'hi');
    // P5: flag only with real ciphertext — plaintext must not claim encrypted
    expect(p.encrypted).toBe(false);
    expect(p.type).toBe('chat');
    expect(p.to).toBe('bobpk');
    expect(p.text).toBe('hi');
    expect(p.from.length).toBeGreaterThan(0);
    expect(api.isE2EEnabledLocal()).toBe(true);
  });

  it('buildChatPayload.encrypted false when E2E disabled', async () => {
    const api = await import('../src/lib/api.ts');
    api.setIdentity('a'.repeat(64), 'b'.repeat(64));
    api.setE2EEnabledLocal(false);
    const p = api.buildChatPayload('bobpk', 'plain');
    expect(p.encrypted).toBe(false);
    expect(api.isE2EEnabledLocal()).toBe(false);
  });

  it('toggling E2E does not lie: plaintext stays encrypted:false', async () => {
    const api = await import('../src/lib/api.ts');
    api.setIdentity('a'.repeat(64), 'b'.repeat(64));
    api.setE2EEnabledLocal(true);
    expect(api.buildChatPayload('x', 'a').encrypted).toBe(false);
    api.setE2EEnabledLocal(false);
    expect(api.buildChatPayload('x', 'b').encrypted).toBe(false);
    api.setE2EEnabledLocal(true);
    expect(api.buildChatPayload('x', 'c').encrypted).toBe(false);
  });

  it('buildChatPayload.encrypted true only for nip44 ciphertext', async () => {
    const api = await import('../src/lib/api.ts');
    api.setIdentity('a'.repeat(64), 'b'.repeat(64));
    api.setE2EEnabledLocal(true);
    const p = api.buildChatPayload('bobpk', 'nip44:deadbeef');
    expect(p.encrypted).toBe(true);
    expect(p.text.startsWith('nip44:')).toBe(true);
  });
});
