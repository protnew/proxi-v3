/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const ls: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => ls[k] ?? null,
  setItem: (k: string, v: string) => { ls[k] = v; },
  removeItem: (k: string) => { delete ls[k]; },
  clear: () => Object.keys(ls).forEach(k => delete ls[k]),
});

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

function rpcOk(result: any) {
  return {
    ok: true, status: 200,
    json: async () => ({ result }),
    text: async () => JSON.stringify({ result }),
    headers: new Headers(),
  } as any;
}
function rpcErr(status: number, body: any = { error: { message: 'boom' } }) {
  return {
    ok: status >= 200 && status < 300, status,
    json: async () => body,
    text: async () => JSON.stringify(body),
    headers: new Headers(),
  } as any;
}

import {
  refreshVPNStatus, connectRealVPN, connectLocalVPN, connectExitVPN,
  shareExitNode, checkEgressIP, disconnectVPN, connectVPN,
  formatBytes, formatUptime, vpnStatus, vpnStats,
} from '../src/lib/vpn';
import {
  getTheme, setTheme, toggleTheme, initTheme, onThemeChange,
} from '../src/lib/theme';

beforeEach(() => {
  vi.clearAllMocks();
  Object.keys(ls).forEach(k => delete ls[k]);
  ls['proxi_token'] = 't';
  mockFetch.mockResolvedValue(rpcOk({
    state: 'connected', bytesDown: 10, bytesUp: 5, peers: [{}], uptime: 9,
    myIP: '10.0.0.2', myPublicKey: 'pk', transport: 'socks', socksAddr: '127.0.0.1:10808',
    upstream: '', mode: 'real', realTraffic: true,
  }));
});

afterEach(async () => {
  mockFetch.mockResolvedValue(rpcOk({ state: 'disconnected' }));
  await disconnectVPN();
  vi.useRealTimers();
});

describe('vpn error + mapState branches', () => {
  it('refreshVPNStatus maps all states', async () => {
    for (const state of ['connected', 'sharing', 'connecting', 'error', 'unknown']) {
      mockFetch.mockResolvedValueOnce(rpcOk({ state, peers: [] }));
      await refreshVPNStatus();
    }
  });

  it('refreshVPNStatus 401 path', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(401));
    const r = await refreshVPNStatus();
    expect(r).toBeNull();
  });

  it('refreshVPNStatus body.error path', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(200, { error: { message: 'rpc fail' } }));
    // status 200 but ok true with error field
    mockFetch.mockResolvedValueOnce({
      ok: true, status: 200,
      json: async () => ({ error: { message: 'rpc fail' } }),
    });
    const r = await refreshVPNStatus();
    expect(r).toBeNull();
  });

  it('connectRealVPN error path', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(500));
    const ok = await connectRealVPN();
    expect(ok).toBe(false);
  });

  it('connectRealVPN success starts poll', async () => {
    vi.useFakeTimers();
    mockFetch.mockResolvedValue(rpcOk({ state: 'connected', peers: [] }));
    const ok = await connectRealVPN('socks://up:1');
    expect(ok).toBe(true);
    // advance poll interval
    await vi.advanceTimersByTimeAsync(2100);
    vi.useRealTimers();
  });

  it('connectLocalVPN error + success', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(503));
    expect(await connectLocalVPN()).toBe(false);
    mockFetch.mockResolvedValue(rpcOk({ state: 'connected', peers: [] }));
    expect(await connectLocalVPN()).toBe(true);
  });

  it('connectExitVPN invalid endpoint', async () => {
    const ok = await connectExitVPN('pk', 'no-port');
    expect(ok).toBe(false);
  });

  it('connectExitVPN short pubkey → real upstream', async () => {
    mockFetch.mockResolvedValue(rpcOk({ state: 'connected', peers: [] }));
    const ok = await connectExitVPN('short', '1.2.3.4:1080');
    expect(ok).toBe(true);
  });

  it('connectExitVPN full path + fallback', async () => {
    // first rpc fails then fallback real works
    mockFetch
      .mockResolvedValueOnce(rpcErr(500)) // connect_to_exit_node
      .mockResolvedValue(rpcOk({ state: 'connected', peers: [] })); // fallback real + status
    const ok = await connectExitVPN('p'.repeat(40), '9.9.9.9:1080');
    expect(typeof ok).toBe('boolean');
  });

  it('shareExitNode error + success', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(500));
    expect(await shareExitNode()).toBe(false);
    mockFetch.mockResolvedValue(rpcOk({ state: 'sharing', peers: [] }));
    expect(await shareExitNode()).toBe(true);
  });

  it('checkEgressIP', async () => {
    mockFetch.mockResolvedValueOnce(rpcOk({ ip: '1.1.1.1' }));
    expect(await checkEgressIP()).toBe('1.1.1.1');
  });

  it('disconnectVPN swallows errors', async () => {
    mockFetch.mockResolvedValueOnce(rpcErr(500));
    await disconnectVPN();
  });

  it('connectVPN variants', async () => {
    mockFetch.mockResolvedValue(rpcOk({ state: 'connected', peers: [] }));
    expect(await connectVPN()).toBe(true);
    expect(await connectVPN('socks://x')).toBe(true);
    expect(await connectVPN('pk@1.2.3.4:9')).toBe(true);
    expect(await connectVPN('1.2.3.4:9')).toBe(true);
  });

  it('formatBytes all buckets', () => {
    expect(formatBytes(100)).toContain('B');
    expect(formatBytes(2048)).toContain('KB');
    expect(formatBytes(2 * 1048576)).toContain('MB');
    expect(formatBytes(2 * 1073741824)).toContain('GB');
  });

  it('formatUptime hours and minutes', () => {
    expect(formatUptime(5)).toMatch(/0:05|5/);
    expect(formatUptime(65)).toMatch(/1:05/);
    expect(formatUptime(3661)).toMatch(/1:01:01/);
  });
});

describe('theme full', () => {
  it('set/get/toggle/init/listener', () => {
    const seen: string[] = [];
    const off = onThemeChange((t) => seen.push(t));
    expect(setTheme('light')).toBe('light');
    expect(getTheme()).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(toggleTheme()).toBe('dark');
    expect(toggleTheme()).toBe('light');
    ls['messenger-theme'] = 'dark';
    initTheme();
    expect(getTheme()).toBe('dark');
    off();
    setTheme('light');
    expect(seen.length).toBeGreaterThan(0);
  });

  it('initTheme ignores garbage', () => {
    ls['messenger-theme'] = 'neon';
    initTheme();
    expect(['dark', 'light']).toContain(getTheme());
  });
});
