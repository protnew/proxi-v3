import { describe, it, expect, beforeEach, vi } from 'vitest';

const store: Record<string, string> = {};
globalThis.localStorage = {
  getItem: (k: string) => store[k] || null,
  setItem: (k: string, v: string) => { store[k] = v; },
  removeItem: (k: string) => { delete store[k]; },
  clear: () => { Object.keys(store).forEach(k => delete store[k]); },
} as any;

globalThis.fetch = vi.fn() as any;

describe('MSG-009 isE2EEnabled on send path', () => {
  beforeEach(() => {
    Object.keys(store).forEach(k => delete store[k]);
    vi.resetModules();
  });

  it('buildChatPayload.encrypted true by default', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    api.setE2EEnabledLocal(true);
    const p = api.buildChatPayload('bobpk', 'hi');
    expect(p.encrypted).toBe(true);
    expect(p.type).toBe('chat');
    expect(p.to).toBe('bobpk');
    expect(p.text).toBe('hi');
    expect(p.from.length).toBeGreaterThan(0);
  });

  it('buildChatPayload.encrypted false when E2E disabled', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    api.setE2EEnabledLocal(false);
    const p = api.buildChatPayload('bobpk', 'plain');
    expect(p.encrypted).toBe(false);
    expect(api.isE2EEnabledLocal()).toBe(false);
  });

  it('toggling E2E flips subsequent payloads', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    api.setE2EEnabledLocal(true);
    expect(api.buildChatPayload('x', 'a').encrypted).toBe(true);
    api.setE2EEnabledLocal(false);
    expect(api.buildChatPayload('x', 'b').encrypted).toBe(false);
    api.setE2EEnabledLocal(true);
    expect(api.buildChatPayload('x', 'c').encrypted).toBe(true);
  });
});
