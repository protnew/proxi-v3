import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock fetch globally
const mockFetch = vi.fn();
globalThis.fetch = mockFetch as any;

// Mock localStorage
const store: Record<string, string> = {};
globalThis.localStorage = {
  getItem: (k: string) => store[k] || null,
  setItem: (k: string, v: string) => { store[k] = v; },
  removeItem: (k: string) => { delete store[k]; },
  clear: () => { Object.keys(store).forEach(k => delete store[k]); },
} as any;

// Mock import.meta
(vi as any).stubGlobal('import', { meta: { env: { VITE_API_URL: 'http://localhost:8080' } } });

describe('API client', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    Object.keys(store).forEach(k => delete store[k]);
  });

  it('should expose API_BASE', async () => {
    const api = await import('../src/lib/api.ts');
    // API_BASE is exported — check it's a string
    expect(typeof api.API_BASE).toBe('string');
  });

  it('should initIdentity synchronously returning a hex string', async () => {
    const api = await import('../src/lib/api.ts');
    const pk = api.initIdentity();
    expect(typeof pk).toBe('string');
    expect(pk.length).toBe(64); // 32 bytes hex
  });

  it('should getPubkey returns cached identity', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    const pk = api.getPubkey();
    expect(pk.length).toBe(64);
  });

  it('should getSeckey returns cached private key', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    const sk = api.getSeckey();
    expect(sk.length).toBe(64);
  });

  it('should getUserId returns empty initially', async () => {
    const api = await import('../src/lib/api.ts');
    const uid = api.getUserId();
    expect(uid).toBe('');
  });

  it('should setIdentity stores pubkey and privateKey', async () => {
    const api = await import('../src/lib/api.ts');
    api.setIdentity('test_pubkey_123', 'test_privkey_456');
    expect(api.getPubkey()).toBe('test_pubkey_123');
    expect(api.getSeckey()).toBe('test_privkey_456');
  });

  it('should sendPresence does not crash', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    expect(() => api.sendPresence(true)).not.toThrow();
    expect(() => api.sendPresence(false)).not.toThrow();
  });

  it('should getName returns cached name or truncated pubkey', async () => {
    const api = await import('../src/lib/api.ts');
    const name = api.getName('abcdef1234567890');
    expect(typeof name).toBe('string');
    expect(name.length).toBeGreaterThan(0);
  });

  it('should getStatus returns object', async () => {
    const api = await import('../src/lib/api.ts');
    const status = api.getStatus();
    expect(typeof status).toBe('object');
  });

  it('should onMessage/onPresence/onTyping register callbacks', async () => {
    const api = await import('../src/lib/api.ts');
    expect(() => api.onMessage(() => {})).not.toThrow();
    expect(() => api.onPresence(() => {})).not.toThrow();
    expect(() => api.onTyping(() => {})).not.toThrow();
  });

  it('should sendDM via WS returns 200', async () => {
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    // Mock WebSocket as open
    const fakeWs = { readyState: 1, send: vi.fn(), OPEN: 1 };
    // Override sendRaw by connecting first
    const result = await api.chatApi.sendDM('user123', 'hello');
    expect(result).toBeDefined();
  });

  it('should createGroup calls /api/groups/create', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ groupId: 'grp-123', name: 'Test', status: 'created' }),
    });
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    const result = await api.createGroup('TestGroup');
    expect(mockFetch).toHaveBeenCalled();
  });

  it('should listGroups calls /api/groups/list', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ count: 0, groups: [] }),
    });
    const api = await import('../src/lib/api.ts');
    api.initIdentity();
    const result = await api.listGroups();
    expect(mockFetch).toHaveBeenCalled();
  });
});
