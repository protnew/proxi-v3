/**
 * @vitest-environment jsdom
 * Deep coverage for remaining low-coverage modules
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { webcrypto } from 'crypto';

vi.stubGlobal('crypto', webcrypto);

// ── mock fetch ──
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// ── mock localStorage ──
const ls: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => ls[k] ?? null,
  setItem: (k: string, v: string) => { ls[k] = v; },
  removeItem: (k: string) => { delete ls[k]; },
  clear: () => { Object.keys(ls).forEach(k => delete ls[k]); },
});

// ── mock Tauri ──
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn().mockRejectedValue(new Error('not tauri')),
}));

// ── mock Notification ──
class MockNotification {
  static permission = 'granted';
  static requestPermission = vi.fn().mockResolvedValue('granted');
  constructor(public title: string, public opts?: any) {}
}
vi.stubGlobal('Notification', MockNotification as any);

function mockResp(data: any, ok = true, status = 200) {
  return {
    ok, status,
    json: async () => data,
    text: async () => typeof data === 'string' ? data : JSON.stringify(data),
    headers: new Headers(),
  } as any;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockFetch.mockReset();
  mockFetch.mockResolvedValue({
    ok: true, status: 200,
    json: async () => ({}),
    text: async () => '{}',
    headers: new Headers(),
  });
  Object.keys(ls).forEach(k => delete ls[k]);
  ls['proxi_token'] = 'tok';
});

// ═══════════════════════════════════════════════════════════
// DESKTOP
// ═══════════════════════════════════════════════════════════
import {
  getIsDesktop, getDesktopVpnStatus, startDesktopVpn,
  stopDesktopVpn, getSystemInfo, showNotification,
} from '../src/lib/desktop';

describe('desktop.ts — browser fallbacks', () => {
  it('getIsDesktop is false outside Tauri', () => {
    expect(getIsDesktop()).toBe(false);
  });

  it('getDesktopVpnStatus returns unavailable outside Tauri', async () => {
    const r = await getDesktopVpnStatus();
    expect(r).toEqual({ status: 'unavailable', peers: 0, phase: 'idle' });
  });

  it('startDesktopVpn throws outside Tauri', async () => {
    await expect(startDesktopVpn('wg0')).rejects.toThrow(/Not running in Tauri/);
  });

  it('stopDesktopVpn throws outside Tauri', async () => {
    await expect(stopDesktopVpn()).rejects.toThrow(/Not running in Tauri/);
  });

  it('getSystemInfo returns navigator.platform outside Tauri', async () => {
    const r = await getSystemInfo();
    expect(r.os).toBeDefined();
    expect(r.arch).toBe('unknown');
  });

  it('showNotification uses Web Notification fallback', async () => {
    await showNotification('Title', 'Body');
    // no throw
  });

  it('showNotification no-ops when permission not granted', async () => {
    MockNotification.permission = 'denied';
    await showNotification('T', 'B');
    MockNotification.permission = 'granted';
  });
});

// ═══════════════════════════════════════════════════════════
// EXIT NODE
// ═══════════════════════════════════════════════════════════
import { ExitNode } from '../src/lib/exit-node';

describe('exit-node.ts', () => {
  function makeTunnel() {
    const handlers: Record<string, Function> = {};
    return {
      on: vi.fn((ev: string, cb: Function) => { handlers[ev] = cb; }),
      send: vi.fn(),
      close: vi.fn(),
      _emit: (ev: string, data: string) => handlers[ev]?.(data),
    };
  }

  it('addTunnel registers and listens for messages', () => {
    const node = new ExitNode();
    const tunnel = makeTunnel();
    node.addTunnel('peer1', tunnel as any);
    expect(tunnel.on).toHaveBeenCalledWith('message', expect.any(Function));
  });

  it('handleMessage fetches URL and sends response', async () => {
    const node = new ExitNode();
    const tunnel = makeTunnel();
    node.addTunnel('peer1', tunnel as any);
    tunnel.send.mockClear();

    mockFetch.mockReset();
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      text: async () => 'hello body',
      json: async () => ({}),
      headers: new Headers(),
    });

    tunnel._emit('message', JSON.stringify({
      type: 'http-request', id: 'req1', url: 'https://example.com', method: 'GET',
    }));

    await new Promise(r => setTimeout(r, 50));
    expect(mockFetch).toHaveBeenCalled();
    expect(tunnel.send).toHaveBeenCalled();
    const sent = JSON.parse(tunnel.send.mock.calls[0][0]);
    expect(sent.type).toBe('http-response');
    expect(sent.id).toBe('req1');
    expect(sent.body).toBe('hello body');
  });

  it('handleMessage returns 502 on fetch error', async () => {
    const node = new ExitNode();
    const tunnel = makeTunnel();
    node.addTunnel('peer2', tunnel as any);
    tunnel.send.mockClear();

    mockFetch.mockReset();
    mockFetch.mockRejectedValue(new Error('network down'));
    tunnel._emit('message', JSON.stringify({
      type: 'http-request', id: 'req2', url: 'https://fail.example',
    }));
    await new Promise(r => setTimeout(r, 50));
    expect(tunnel.send).toHaveBeenCalled();
    const payloads = tunnel.send.mock.calls.map((c: any[]) => {
      try { return JSON.parse(c[0]); } catch { return null; }
    }).filter(Boolean);
    expect(payloads.some((p: any) => p.status === 502)).toBe(true);
  });

  it('handleMessage ignores non-JSON and unknown types', async () => {
    const node = new ExitNode();
    const tunnel = makeTunnel();
    node.addTunnel('peer1', tunnel as any);
    tunnel._emit('message', 'not-json');
    tunnel._emit('message', JSON.stringify({ type: 'other' }));
    await new Promise(r => setTimeout(r, 10));
    expect(tunnel.send).not.toHaveBeenCalled();
  });

  it('removeTunnel closes and deletes', () => {
    const node = new ExitNode();
    const tunnel = makeTunnel();
    node.addTunnel('peer1', tunnel as any);
    node.removeTunnel('peer1');
    expect(tunnel.close).toHaveBeenCalled();
  });

  it('removeTunnel on unknown peer is safe', () => {
    const node = new ExitNode();
    expect(() => node.removeTunnel('unknown')).not.toThrow();
  });
});

// ═══════════════════════════════════════════════════════════
// API-EXTENDED
// ═══════════════════════════════════════════════════════════
import {
  searchMessages, editMessage, deleteMessage,
  isE2EEnabled, setE2EEnabled, loadE2EPref,
  createGroupUI, listAllGroups, toggleVPN, getVPNStatus,
} from '../src/lib/api-extended';

describe('api-extended.ts', () => {
  it('searchMessages calls /api/search', async () => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(mockResp({ results: [] }));
    const r = await searchMessages('hello', 10);
    expect(mockFetch).toHaveBeenCalled();
    const url = String(mockFetch.mock.calls[0][0]);
    expect(url).toContain('/api/search');
    expect(url).toContain('hello');
    expect(r.status === 200 || r.status === 0).toBe(true);
  });

  it('editMessage POSTs to /api/messages/edit', async () => {
    mockFetch.mockResolvedValueOnce(mockResp({ ok: true }));
    await editMessage('m1', 'new text');
    expect(mockFetch.mock.calls[0][0]).toContain('/api/messages/edit');
  });

  it('deleteMessage POSTs to /api/messages/delete', async () => {
    mockFetch.mockResolvedValueOnce(mockResp({ ok: true }));
    await deleteMessage('m1');
    expect(mockFetch.mock.calls[0][0]).toContain('/api/messages/delete');
  });

  it('E2E toggle round-trip + localStorage', () => {
    expect(isE2EEnabled()).toBe(true);
    setE2EEnabled(false);
    expect(isE2EEnabled()).toBe(false);
    expect(ls['proxi_e2e']).toBe('0');
    setE2EEnabled(true);
    expect(ls['proxi_e2e']).toBe('1');
    expect(loadE2EPref()).toBe(true);
  });

  it('loadE2EPref reads 0 as disabled', () => {
    ls['proxi_e2e'] = '0';
    expect(loadE2EPref()).toBe(false);
  });

  it('createGroupUI delegates to createGroup', async () => {
    mockFetch.mockResolvedValueOnce(mockResp({ id: 'g1' }));
    await createGroupUI('G', ['a', 'b']);
    expect(mockFetch).toHaveBeenCalled();
  });

  it('listAllGroups delegates to listGroups', async () => {
    mockFetch.mockResolvedValueOnce(mockResp([]));
    await listAllGroups();
    expect(mockFetch).toHaveBeenCalled();
  });

  it('toggleVPN connect/disconnect', async () => {
    mockFetch.mockResolvedValueOnce(mockResp({ ok: true }));
    await toggleVPN(true);
    expect(JSON.parse(mockFetch.mock.calls[0][1].body).action).toBe('connect');
    mockFetch.mockResolvedValueOnce(mockResp({ ok: true }));
    await toggleVPN(false);
    expect(JSON.parse(mockFetch.mock.calls[1][1].body).action).toBe('disconnect');
  });

  it('getVPNStatus queries status', async () => {
    mockFetch.mockResolvedValueOnce(mockResp({ state: 'off' }));
    await getVPNStatus();
    expect(mockFetch.mock.calls[0][0]).toContain('vpn/rpc');
  });
});

// ═══════════════════════════════════════════════════════════
// FILE TRANSFER
// ═══════════════════════════════════════════════════════════
import {
  createManifest, reassemble, sendChunks, FileReceiver,
} from '../src/lib/file-transfer';

describe('file-transfer.ts deep', () => {
  function makeFile(content: string, name = 't.txt', type = 'text/plain'): File {
    const blob = new Blob([content], { type });
    return new File([blob], name, { type });
  }

  it('createManifest splits file and hashes', async () => {
    const file = makeFile('hello world content for transfer');
    const { manifest, chunks } = await createManifest(file, 'sender-pub');
    expect(manifest.name).toBe('t.txt');
    expect(manifest.from).toBe('sender-pub');
    expect(manifest.size).toBe(file.size);
    expect(manifest.hash).toHaveLength(64);
    expect(chunks.length).toBe(manifest.chunks);
    expect(manifest.chunks).toBeGreaterThanOrEqual(1);
  });

  it('createManifest handles empty-ish small file', async () => {
    const file = makeFile('x');
    const { manifest, chunks } = await createManifest(file, 's');
    expect(chunks.length).toBe(1);
    expect(manifest.mimeType).toBe('text/plain');
  });

  it('reassemble recovers original content and validates hash', async () => {
    const content = 'ABCDEFGHIJ'.repeat(100);
    const file = makeFile(content);
    const { manifest, chunks } = await createManifest(file, 's');
    const map = new Map<number, ArrayBuffer>();
    chunks.forEach((c, i) => map.set(i, c));
    const { blob, valid } = await reassemble(manifest, map);
    expect(valid).toBe(true);
    const text = await blob.text();
    expect(text).toBe(content);
  });

  it('reassemble detects hash mismatch', async () => {
    const file = makeFile('original');
    const { manifest, chunks } = await createManifest(file, 's');
    const bad = new Map<number, ArrayBuffer>();
    bad.set(0, new TextEncoder().encode('tampered').buffer);
    // force wrong chunk count alignment
    const fakeManifest = { ...manifest, chunks: 1 };
    const { valid } = await reassemble(fakeManifest, bad);
    expect(valid).toBe(false);
  });

  it('sendChunks sends manifest + chunks + complete', async () => {
    const file = makeFile('chunk-me-please');
    const { manifest, chunks } = await createManifest(file, 's');
    const sent: any[] = [];
    const dc = {
      readyState: 'open',
      bufferedAmount: 0,
      onopen: null as any,
      send: (d: any) => sent.push(d),
    };
    const progress: number[] = [];
    await sendChunks(dc as any, manifest, chunks, (s, t) => progress.push(s));
    // first is manifest JSON
    expect(JSON.parse(sent[0]).type).toBe('file-manifest');
    // last is complete
    expect(JSON.parse(sent[sent.length - 1]).type).toBe('file-complete');
    expect(progress.length).toBe(chunks.length);
  });

  it('sendChunks waits if channel not open then times out open', async () => {
    const file = makeFile('x');
    const { manifest, chunks } = await createManifest(file, 's');
    const dc = {
      readyState: 'connecting',
      bufferedAmount: 0,
      onopen: null as any,
      send: vi.fn(),
    };
    // open after 20ms
    setTimeout(() => {
      (dc as any).readyState = 'open';
      dc.onopen?.();
    }, 20);
    await sendChunks(dc as any, manifest, chunks);
    expect(dc.send).toHaveBeenCalled();
  });

  it('FileReceiver full cycle: manifest → chunks → complete', async () => {
    const content = 'receiver-test-data-12345';
    const file = makeFile(content);
    const { manifest, chunks } = await createManifest(file, 'sender');

    const receiver = new FileReceiver();
    let completed: { blob: Blob; manifest: any } | null = null;
    let progress = 0;
    receiver.register(manifest.id, {
      onProgress: (r) => { progress = r; },
      onComplete: (blob, m) => { completed = { blob, manifest: m }; },
      onError: () => {},
    });

    // inject manifest
    receiver.handleMessage(JSON.stringify({ type: 'file-manifest', manifest }));
    // inject chunks
    chunks.forEach((c, i) => receiver.handleChunk(manifest.id, i, c));
    // complete
    receiver.handleMessage(JSON.stringify({ type: 'file-complete', manifestId: manifest.id }));

    await new Promise(r => setTimeout(r, 50));
    expect(completed).not.toBeNull();
    expect(progress).toBe(chunks.length);
    const text = await completed!.blob.text();
    expect(text).toBe(content);
  });

  it('FileReceiver onError on hash mismatch', async () => {
    const file = makeFile('good');
    const { manifest } = await createManifest(file, 's');
    const receiver = new FileReceiver();
    let err = '';
    receiver.register(manifest.id, {
      onError: (e) => { err = e; },
    });
    receiver.handleMessage(JSON.stringify({ type: 'file-manifest', manifest }));
    receiver.handleChunk(manifest.id, 0, new TextEncoder().encode('BAD').buffer);
    receiver.handleMessage(JSON.stringify({ type: 'file-complete', manifestId: manifest.id }));
    await new Promise(r => setTimeout(r, 50));
    expect(err).toContain('Hash');
  });

  it('FileReceiver ignores garbage strings', () => {
    const receiver = new FileReceiver();
    expect(() => receiver.handleMessage('not-json')).not.toThrow();
    expect(() => receiver.handleMessage(new ArrayBuffer(4))).not.toThrow();
  });
});

// ═══════════════════════════════════════════════════════════
// IPC
// ═══════════════════════════════════════════════════════════
import { ipc } from '../src/lib/ipc';

describe('ipc.ts deep', () => {
  it('register + call falls back to handler when Tauri fails', async () => {
    ipc.register('test_echo', async (p) => ({ echo: p }));
    const r = await ipc.call('test_echo', { a: 1 });
    expect(r).toEqual({ echo: { a: 1 } });
  });

  it('call throws for unknown method', async () => {
    await expect(ipc.call('no_such_method_xyz')).rejects.toThrow(/No handler/);
  });

  it('dispatch returns JSON-RPC envelope', async () => {
    ipc.register('add', async (p) => p.x + p.y);
    const r = await ipc.dispatch({ method: 'add', params: { x: 2, y: 3 }, id: '1' });
    expect(r).toEqual({ jsonrpc: '2.0', result: 5, id: '1' });
  });

  it('built-in get_online_users handler uses fetch', async () => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(mockResp({ users: ['a'] }));
    const r = await ipc.call('get_online_users');
    expect(r).toEqual({ users: ['a'] });
    expect(String(mockFetch.mock.calls[0][0])).toContain('/api/v1/chat/online');
  });

  it('built-in send_message handler POSTs', async () => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(mockResp({ ok: true }));
    const r = await ipc.call('send_message', { to: 'bob', content: 'hi' });
    expect(r).toEqual({ ok: true });
    expect(mockFetch.mock.calls[0][1].method).toBe('POST');
  });

  it('built-in get_content_list handler', async () => {
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(mockResp({ items: [] }));
    const r = await ipc.call('get_content_list');
    expect(r).toEqual({ items: [] });
  });
});
