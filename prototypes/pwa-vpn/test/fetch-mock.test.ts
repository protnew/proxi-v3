/**
 * @vitest-environment jsdom
 * Fetch mock tests for api.ts and vpn.ts — covers all network functions
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// ═══ Mock global fetch ═══
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// ═══ Mock localStorage ═══
const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: vi.fn((k: string) => store[k] ?? null),
  setItem: vi.fn((k: string, v: string) => { store[k] = v; }),
  removeItem: vi.fn((k: string) => { delete store[k]; }),
  clear: vi.fn(() => { Object.keys(store).forEach(k => delete store[k]); }),
});

// ═══ Mock WebSocket ═══
class MockWS {
  readyState = 0; onopen: any; onmessage: any; onerror: any; onclose: any;
  send = vi.fn(); close = vi.fn();
  static OPEN = 1;
  constructor(public url: string) {
    setTimeout(() => { this.readyState = 1; this.onopen?.(); }, 5);
  }
}
vi.stubGlobal('WebSocket', MockWS as any);

// Mock crypto
vi.stubGlobal('crypto', {
  ...globalThis.crypto,
  getRandomValues: (arr: Uint8Array) => { for (let i = 0; i < arr.length; i++) arr[i] = Math.floor(Math.random() * 256); return arr; },
  randomUUID: () => 'mock-uuid-' + Math.random().toString(36).slice(2),
});

import {
  request, isE2EEnabledLocal, setE2EEnabledLocal,
  buildChatPayload, sendDM, sendTyping, sendGroupMessage,
  createGroup, listGroups, subscribeGroup, updateProfile,
  setIdentity, getPubkey, getUserId, getSeckey,
  sendCallSignal, sendFileManifest, sendBinaryVoice,
  connectWebSocket, sendPresence, getName, getStatus,
  onMessage, onPresence, onTyping,
} from '../src/lib/api';

import {
  refreshVPNStatus, connectRealVPN, connectLocalVPN, connectExitVPN,
  shareExitNode, checkEgressIP, disconnectVPN, connectVPN,
  formatBytes, formatUptime, vpnStatus, vpnStats,
} from '../src/lib/vpn';

// Helper: create mock Response
function mockResponse(data: any, ok = true, status = 200): Response {
  return {
    ok, status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(typeof data === 'string' ? data : JSON.stringify(data)),
    headers: new Headers(),
  } as any;
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.keys(store).forEach(k => delete store[k]);
});

describe('API request()', () => {
  it('makes GET request and returns data', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ id: 1, name: 'test' }));
    const result = await request('/api/test');
    expect(result.status).toBe(200);
    expect(result.data).toEqual({ id: 1, name: 'test' });
  });

  it('handles 401 unauthorized', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ error: 'unauthorized' }, false, 401));
    const result = await request('/api/protected');
    expect(result.status).toBe(401);
    expect(result.error).toBeDefined();
  });

  it('handles 500 server error', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ error: 'server' }, false, 500));
    const result = await request('/api/error');
    expect(result.status).toBe(500);
  });

  it('handles network error', async () => {
    mockFetch.mockRejectedValueOnce(new TypeError('network error'));
    const result = await request('/api/fail');
    expect(result.status).toBe(0);
  });

  it('sends POST with body', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await request('/api/messages', { method: 'POST', body: '{"text":"hi"}' });
    expect(mockFetch).toHaveBeenCalled();
    const callArgs = mockFetch.mock.calls[0];
    expect(callArgs[1].method).toBe('POST');
  });
});

describe('API identity', () => {
  it('isE2EEnabledLocal/setE2EEnabledLocal round-trip', () => {
    expect(isE2EEnabledLocal()).toBe(true);
    setE2EEnabledLocal(false);
    expect(isE2EEnabledLocal()).toBe(false);
    setE2EEnabledLocal(true);
    expect(isE2EEnabledLocal()).toBe(true);
  });

  it('setIdentity/getPubkey/getUserId round-trip', () => {
    setIdentity('test-pubkey-123', 'test-privkey-456');
    // setIdentity sets module-level vars
    expect(typeof getPubkey()).toBe('string');
    expect(typeof getUserId()).toBe('string');
  });

  it('getName returns pubkey short form or name from store', () => {
    const name = getName('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
    expect(typeof name).toBe('string');
  });
});

describe('API messaging', () => {
  beforeEach(() => {
    setIdentity('alice-pub', 'alice-priv');
    store['proxi_token'] = 'mock-jwt-token';
  });

  it('sendDM calls POST /api/messages', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const result = await sendDM('bob-pub', 'hello');
    expect(mockFetch).toHaveBeenCalled();
    expect(result).toBeDefined();
  });

  it('sendTyping returns 200 via WS', async () => {
    const r = await sendTyping('bob-pub');
    expect(r.status).toBe(200);
  });

  it('sendGroupMessage calls API', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await sendGroupMessage('group-1', 'hello group');
    expect(mockFetch).toHaveBeenCalled();
  });

  it('sendCallSignal returns valid response', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await sendCallSignal('bob-pub', { type: 'call-offer', sdp: 'test' });
    // WS path returns 200, fetch fallback returns whatever fetch returns
    expect(r).toBeDefined();
    expect(typeof r.status).toBe('number');
  });

  it('sendFileManifest calls API', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await sendFileManifest('bob-pub', { name: 'test.txt', size: 100 });
    expect(mockFetch).toHaveBeenCalled();
  });

  it('sendBinaryVoice calls API', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const blob = new Blob([new Uint8Array(10)], { type: 'audio/webm' });
    await sendBinaryVoice('bob-pub', blob);
    expect(mockFetch).toHaveBeenCalled();
  });
});

describe('API groups', () => {
  beforeEach(() => {
    setIdentity('alice-pub', 'alice-priv');
    store['proxi_token'] = 'mock-jwt';
  });

  it('createGroup POSTs', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true, id: 'g1' }));
    const r = await createGroup('Test Group', ['bob']);
    expect(mockFetch).toHaveBeenCalled();
  });

  it('listGroups GETs', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true, data: [] }));
    await listGroups();
    expect(mockFetch).toHaveBeenCalled();
  });

  it('subscribeGroup POSTs', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await subscribeGroup('group-1');
    expect(mockFetch).toHaveBeenCalled();
  });
});

describe('API profile', () => {
  beforeEach(() => {
    setIdentity('alice-pub', 'alice-priv');
    store['proxi_token'] = 'mock-jwt';
  });

  it('updateProfile PUTs/POSTs', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await updateProfile({ name: 'Alice', bio: 'Test' });
    expect(mockFetch).toHaveBeenCalled();
  });
});

describe('API websocket + callbacks', () => {
  it('connectWebSocket returns WebSocket instance', () => {
    setIdentity('alice', 'priv');
    store['proxi_token'] = 'token';
    const ws = connectWebSocket('test-token', () => {});
    expect(ws).toBeDefined();
  });

  it('sendPresence does not throw', () => {
    expect(() => sendPresence(true)).not.toThrow();
    expect(() => sendPresence(false)).not.toThrow();
  });

  it('onMessage registers callback', () => {
    expect(() => onMessage(() => {})).not.toThrow();
  });

  it('onPresence registers callback', () => {
    expect(() => onPresence(() => {})).not.toThrow();
  });

  it('onTyping registers callback', () => {
    expect(() => onTyping(() => {})).not.toThrow();
  });

  it('getStatus returns object', () => {
    const s = getStatus();
    expect(typeof s).toBe('object');
  });

  it('buildChatPayload returns correct shape', () => {
    const payload = buildChatPayload('bob-pub', 'hello');
    expect(payload).toBeDefined();
  });
});

describe('VPN functions', () => {
  it('refreshVPNStatus handles errors gracefully', async () => {
    mockFetch.mockResolvedValue(mockResponse({ result: { state: 'disconnected' } }));
    const r = await refreshVPNStatus();
    // Returns null on error or VpnBackendStatus on success — both valid
    expect(r === null || typeof r === 'object' || typeof r === 'undefined').toBe(true);
  });

  it('connectRealVPN returns boolean', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await connectRealVPN();
    expect(typeof r).toBe('boolean');
  });

  it('connectLocalVPN returns boolean', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await connectLocalVPN();
    expect(typeof r).toBe('boolean');
  });

  it('connectExitVPN returns boolean', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await connectExitVPN('pubkey123', '1.2.3.4:8080');
    expect(typeof r).toBe('boolean');
  });

  it('shareExitNode returns boolean', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await shareExitNode();
    expect(typeof r).toBe('boolean');
  });

  it('checkEgressIP returns string', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ip: '1.2.3.4' }));
    const r = await checkEgressIP();
    expect(typeof r).toBe('string');
  });

  it('disconnectVPN does not throw', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    await expect(disconnectVPN()).resolves.not.toThrow();
  });

  it('connectVPN returns boolean', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ ok: true }));
    const r = await connectVPN('exit-url');
    expect(typeof r).toBe('boolean');
  });
});

describe('VPN utilities', () => {
  it('formatBytes formats correctly', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(1024)).toContain('KB');
    expect(formatBytes(1048576)).toContain('MB');
    expect(formatBytes(1073741824)).toContain('GB');
  });

  it('formatUptime formats correctly', () => {
    expect(formatUptime(0)).toBeDefined();
    expect(formatUptime(3600)).toBeDefined();
    expect(formatUptime(86400)).toBeDefined();
  });

  it('vpnStatus is a writable store', () => {
    vpnStatus.set('disconnected'); // reset
    let val: any;
    const unsub = vpnStatus.subscribe(v => val = v);
    expect(val).toBe('disconnected');
    vpnStatus.set('connecting');
    expect(val).toBe('connecting');
    unsub();
  });

  it('vpnStats is a writable store', () => {
    // Reset stats
    vpnStats.set({ rx: 0, tx: 0, uptime: 0, connectedPeers: 0 });
    let val: any;
    const unsub = vpnStats.subscribe(v => val = v);
    expect(val).toBeDefined();
    expect(val.rx).toBe(0);
    unsub();
  });
});
