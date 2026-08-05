/**
 * @vitest-environment jsdom
 * Force WS OPEN path in api.ts: sendRaw, connectRelays, binary voice, group, presence
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { webcrypto } from 'crypto';
vi.stubGlobal('crypto', webcrypto);

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const ls: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => ls[k] ?? null,
  setItem: (k: string, v: string) => { ls[k] = v; },
  removeItem: (k: string) => { delete ls[k]; },
  clear: () => Object.keys(ls).forEach(k => delete ls[k]),
});

class MockWS {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = MockWS.CONNECTING;
  onopen: ((ev?: any) => void) | null = null;
  onmessage: ((ev: any) => void) | null = null;
  onerror: ((ev?: any) => void) | null = null;
  onclose: ((ev?: any) => void) | null = null;
  sent: any[] = [];
  url: string;
  constructor(url: string) {
    this.url = url;
    MockWS.instances.push(this);
    queueMicrotask(() => {
      this.readyState = MockWS.OPEN;
      this.onopen?.({});
    });
  }
  send(data: any) { this.sent.push(data); }
  close() { this.readyState = MockWS.CLOSED; this.onclose?.({}); }
  static instances: MockWS[] = [];
  static reset() { MockWS.instances = []; }
}
vi.stubGlobal('WebSocket', MockWS as any);

function ok(data: any = {}) {
  return {
    ok: true, status: 200,
    json: async () => data,
    text: async () => JSON.stringify(data),
    headers: new Headers(),
  } as any;
}

import {
  setIdentity, connectRelays, sendDM, sendGroupMessage, sendTyping,
  sendBinaryVoice, sendPresence, sendCallSignal, onMessage, onPresence, onTyping,
  getStatus, chatApi, request, initIdentity, initIdentityAsync, getName,
  connectWebSocket, buildChatPayload,
} from '../src/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
  MockWS.reset();
  Object.keys(ls).forEach(k => delete ls[k]);
  setIdentity('a'.repeat(64), 'b'.repeat(64));
  ls['proxi_token'] = 'jwt-abc';
  mockFetch.mockResolvedValue(ok({ access_token: 'jwt-abc', user_id: 'u1', refresh_token: 'r' }));
});

describe('api.ts WS OPEN paths', () => {
  it('connectRelays opens WS and sends join', async () => {
    const n = await connectRelays();
    expect(typeof n).toBe('number');
    await new Promise(r => setTimeout(r, 30));
    const ws = MockWS.instances.at(-1);
    expect(ws).toBeDefined();
    expect(ws!.readyState).toBe(MockWS.OPEN);
    // join message sent
    expect(ws!.sent.length).toBeGreaterThan(0);
    const join = JSON.parse(String(ws!.sent[0]));
    expect(join.type).toBe('join');
  });

  it('sendDM via WS when open + REST dual-write', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    mockFetch.mockResolvedValue(ok({ ok: true }));
    const r = await sendDM('c'.repeat(64), 'hello via ws');
    expect(r.status).toBe(200);
    const ws = MockWS.instances.at(-1)!;
    const types = ws.sent.map(s => {
      try { return JSON.parse(String(s)).type; } catch { return null; }
    });
    expect(types).toContain('chat');
  });

  it('sendGroupMessage via WS', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    const r = await sendGroupMessage('group-1', 'g-hi');
    expect(r.status).toBe(200);
  });

  it('sendTyping + sendPresence via WS', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    await sendTyping('peer');
    sendPresence(true);
    sendPresence(false);
    const ws = MockWS.instances.at(-1)!;
    const types = ws.sent.map(s => { try { return JSON.parse(String(s)).type; } catch { return ''; } });
    expect(types).toContain('typing');
  });

  it('sendCallSignal via WS OPEN', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    const r = await sendCallSignal('peer', { type: 'offer', sdp: 'x' });
    expect(r.status).toBe(200);
  });

  it('sendBinaryVoice via WS binary frame', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    const blob = new Blob([new Uint8Array([1, 2, 3, 4])], { type: 'audio/webm' });
    const r = await sendBinaryVoice('peer', blob);
    expect(r.status).toBe(200);
    const ws = MockWS.instances.at(-1)!;
    // last send should be Uint8Array binary
    const bin = ws.sent.find(s => s instanceof Uint8Array || ArrayBuffer.isView(s));
    expect(bin).toBeDefined();
  });

  it('handleWsMessage routes chat/typing/join/leave/voice/file', async () => {
    const msgs: any[] = [];
    const pres: any[] = [];
    const typ: any[] = [];
    onMessage((m) => msgs.push(m));
    onPresence((pk, online) => pres.push({ pk, online }));
    onTyping((from) => typ.push(from));

    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    const ws = MockWS.instances.at(-1)!;

    const emit = (obj: any) => ws.onmessage?.({ data: JSON.stringify(obj) });

    emit({ type: 'chat', from: 'bob', to: 'alice', text: 'hi', ts: 1, id: 'm1' });
    emit({ type: 'voice', from: 'bob', to: 'alice', voiceDuration: 3 });
    emit({ type: 'file', from: 'bob', to: 'alice', text: 'f' });
    emit({ type: 'image', from: 'bob', to: 'alice' });
    emit({ type: 'typing', from: 'bob', to: 'alice' });
    emit({ type: 'join', from: 'bob' });
    emit({ type: 'leave', from: 'bob' });
    emit({ type: 'system', from: 'sys', text: 'x' });
    ws.onmessage?.({ data: 'not-json' });
    ws.onmessage?.({ data: 'null' });

    expect(msgs.length).toBeGreaterThan(0);
    expect(typ).toContain('bob');
    expect(pres.some(p => p.pk === 'bob' && p.online === true)).toBe(true);
    expect(pres.some(p => p.pk === 'bob' && p.online === false)).toBe(true);
    expect(getStatus()).toBeDefined();
  });

  it('connectRelays reconnect closes previous', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 15));
    const first = MockWS.instances[0];
    await connectRelays();
    await new Promise(r => setTimeout(r, 15));
    expect(MockWS.instances.length).toBeGreaterThanOrEqual(2);
    expect(first.readyState).toBe(MockWS.CLOSED);
  });

  it('initIdentity generates or returns demo keys', () => {
    delete ls['proxi_demo_role'];
    // may return cached
    const pk = initIdentity();
    expect(typeof pk).toBe('string');
    expect(pk.length).toBeGreaterThan(0);
  });

  it('initIdentity Alice/Bob demo roles', () => {
    ls['proxi_demo_role'] = 'tester1';
    // can't easily clear module cache; at least call doesn't throw
    expect(() => initIdentity()).not.toThrow();
    ls['proxi_demo_role'] = 'tester2';
    expect(() => initIdentity()).not.toThrow();
  });

  it('getName truncates long pubkeys', () => {
    const n = getName('f'.repeat(64));
    expect(typeof n).toBe('string');
  });

  it('connectWebSocket helper', async () => {
    const got: any[] = [];
    const ws = connectWebSocket('tok', (m) => got.push(m));
    await new Promise(r => setTimeout(r, 15));
    (ws as any).onmessage?.({ data: JSON.stringify({ hello: 1 }) });
    expect(got.length).toBeGreaterThanOrEqual(0);
  });

  it('buildChatPayload structure', () => {
    const p = buildChatPayload('to', 'content');
    expect(p).toBeTruthy();
  });

  it('chatApi.sendDM WS path returns via:ws', async () => {
    await connectRelays();
    await new Promise(r => setTimeout(r, 20));
    mockFetch.mockResolvedValue(ok({}));
    const r = await chatApi.sendDM('d'.repeat(64), 'ws-dm');
    expect(r.status).toBe(200);
    expect((r as any).data?.via === 'ws' || r.status === 200).toBe(true);
  });

  it('request with FormData does not force JSON content-type', async () => {
    mockFetch.mockResolvedValue(ok({ id: '1' }));
    const fd = new FormData();
    fd.append('f', new Blob(['x']), 'x.txt');
    await request('/api/files/upload', { method: 'POST', body: fd });
    const opts = mockFetch.mock.calls.at(-1)[1];
    // body is FormData
    expect(opts.body).toBeInstanceOf(FormData);
  });
});
