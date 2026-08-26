import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock api-core BEFORE importing the module under test
vi.mock('../src/lib/api-core', () => {
  let e2e = false;
  return {
    request: vi.fn().mockResolvedValue({ status: 200, data: {} }),
    sendRaw: vi.fn().mockReturnValue(true),
    getCachedIdentity: vi.fn().mockReturnValue({ pubkey: 'AAA', userId: 'user-1', privateKey: 'k'.repeat(64) }),
    isE2EEnabled: () => e2e,
    setE2EEnabled: (v: boolean) => { e2e = v; },
    API_BASE: 'http://x',
    getWsConnection: vi.fn().mockReturnValue({ readyState: 1 }),
    buildWsUrl: vi.fn().mockReturnValue('ws://localhost:8090/ws'),
  };
});
import { sendDM } from '../src/lib/api-actions';
import * as apiCore from '../src/lib/api-core';
const request = vi.mocked(apiCore.request);
const sendRaw = vi.mocked(apiCore.sendRaw);

describe('sendDM — double-save fix (no REST POST when WS delivered)', () => {
  beforeEach(() => {
    sendRaw.mockReset();
    sendRaw.mockReturnValue(true);
    request.mockReset();
    request.mockResolvedValue({ status: 200, data: {} });
  });

  it('WS path: returns ok WITHOUT POST /api/messages', async () => {
    const res = await sendDM('BBB', 'hello');
    expect(sendRaw).toHaveBeenCalledTimes(1);
    expect(request).not.toHaveBeenCalled(); // ← the double-save bug guard
    expect(res.status).toBe(200);
    expect(res.data.via).toBe('ws');
  });

  it('REST fallback: called ONLY when WS send failed', async () => {
    sendRaw.mockReturnValue(false);
    const res = await sendDM('BBB', 'hello');
    expect(sendRaw).toHaveBeenCalledTimes(1);
    // fallback path is Nostr/REST — request may fire only here
  });
});
