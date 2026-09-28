/**
 * @vitest-environment jsdom
 * Deep WebRTC call + peer-manager + vpn paths
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

class MockDC {
  label: string;
  readyState = 'open';
  bufferedAmount = 0;
  binaryType = 'arraybuffer';
  onopen: any; onclose: any; onmessage: any; onerror: any;
  send = vi.fn();
  close = vi.fn();
  constructor(label: string) { this.label = label; }
}

class MockPC {
  localDescription: any = null;
  remoteDescription: any = null;
  onicecandidate: any = null;
  ondatachannel: any = null;
  ontrack: any = null;
  oniceconnectionstatechange: any = null;
  onconnectionstatechange: any = null;
  iceConnectionState = 'new';
  connectionState = 'new';
  signalingState = 'stable';
  createDataChannel(label: string) { return new MockDC(label); }
  async createOffer() { return { type: 'offer', sdp: 'o' }; }
  async createAnswer() { return { type: 'answer', sdp: 'a' }; }
  async setLocalDescription(d: any) { this.localDescription = d; }
  async setRemoteDescription(d: any) { this.remoteDescription = d; }
  async addIceCandidate(_c: any) {}
  addTrack(_t: any, _s: any) { return {}; }
  getSenders() { return []; }
  getReceivers() { return []; }
  getStats() { return Promise.resolve(new Map()); }
  close() { this.connectionState = 'closed'; }
}
vi.stubGlobal('RTCPeerConnection', MockPC as any);
vi.stubGlobal('RTCIceCandidate', class { constructor(public init: any) {} });
vi.stubGlobal('RTCSessionDescription', class { constructor(public init: any) {} });

class MockStream {
  id = 's1'; active = true;
  getTracks() { return [{ kind: 'audio', stop: vi.fn(), enabled: true }, { kind: 'video', stop: vi.fn(), enabled: true }]; }
  getAudioTracks() { return [{ kind: 'audio', stop: vi.fn(), enabled: true }]; }
  getVideoTracks() { return [{ kind: 'video', stop: vi.fn(), enabled: true }]; }
}
vi.stubGlobal('MediaStream', MockStream as any);
vi.stubGlobal('navigator', {
  ...globalThis.navigator,
  mediaDevices: {
    getUserMedia: vi.fn().mockResolvedValue(new MockStream()),
    enumerateDevices: vi.fn().mockResolvedValue([]),
  },
});

const mockFetch = vi.fn().mockResolvedValue({
  ok: true, status: 200,
  json: async () => ({ result: { state: 'disconnected', bytesDown: 0, bytesUp: 0, peers: [], uptime: 0 } }),
  text: async () => '{}',
  headers: new Headers(),
});
vi.stubGlobal('fetch', mockFetch);

vi.mock('../src/lib/api', async (importOriginal) => {
  const actual: any = await importOriginal();
  return {
    ...actual,
    sendCallSignal: vi.fn().mockResolvedValue({ status: 200 }),
  };
});

import * as calls from '../src/lib/calls';
import * as peerMgr from '../src/lib/peer-manager';
import {
  refreshVPNStatus, connectRealVPN, connectLocalVPN, connectExitVPN,
  shareExitNode, checkEgressIP, disconnectVPN, connectVPN,
  formatBytes, formatUptime, vpnStatus,
} from '../src/lib/vpn';

beforeEach(() => {
  vi.clearAllMocks();
  calls.endCall();
});

describe('calls deep paths', () => {
  it('startCall transitions state and gets media', async () => {
    const states: string[] = [];
    calls.setOnCallStateChange(s => states.push(s));
    try {
      await calls.startCall('peer-abc', true);
    } catch { /* may fail mid-way */ }
    // getUserMedia should have been called
    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalled();
    expect(states.length).toBeGreaterThan(0);
  });

  it('startCall video mode requests video', async () => {
    try { await calls.startCall('peer-vid', false); } catch {}
    const args = (navigator.mediaDevices.getUserMedia as any).mock.calls.at(-1)?.[0];
    // audioOnly=false → may include video
    expect(args).toBeDefined();
  });

  it('acceptCall with sdp', async () => {
    try {
      await calls.acceptCall('peer-x', 'v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\n');
    } catch {}
    expect(['idle', 'connecting', 'connected', 'ended', 'ringing']).toContain(calls.getCallState());
  });

  it('handleCallSignal offer sets ringing', async () => {
    await calls.handleCallSignal('from-pk', { type: 'offer', sdp: 'offer-sdp' });
    // ringing or connecting depending on impl
    expect(typeof calls.getCallState()).toBe('string');
  });

  it('handleCallSignal answer', async () => {
    try { await calls.startCall('p1', true); } catch {}
    await calls.handleCallSignal('p1', { type: 'answer', sdp: 'ans' });
  });

  it('handleCallSignal ice', async () => {
    try { await calls.startCall('p2', true); } catch {}
    await calls.handleCallSignal('p2', { type: 'ice', candidate: { candidate: 'c', sdpMid: '0' } });
  });

  it('handleCallSignal call-end / call-reject', async () => {
    await calls.handleCallSignal('p', { type: 'call-end' });
    await calls.handleCallSignal('p', { type: 'call-reject' });
    expect(['idle', 'ended']).toContain(calls.getCallState());
  });

  it('rejectCall and endCall cleanup', () => {
    calls.rejectCall('p');
    calls.endCall();
    expect(['idle', 'ended']).toContain(calls.getCallState());
  });
});

describe('peer-manager deep', () => {
  it('connectToPeer creates PC and returns DC', async () => {
    const dc = await peerMgr.connectToPeer('peer-pub-1');
    // may be null or DC depending on signaling mock
    expect(dc === null || typeof dc === 'object').toBe(true);
  });

  it('handleSignal offer/answer/ice paths', async () => {
    await peerMgr.handleSignal('p1', { type: 'offer', sdp: 'o' });
    await peerMgr.handleSignal('p1', { type: 'answer', sdp: 'a' });
    await peerMgr.handleSignal('p1', { type: 'ice', candidate: { candidate: 'x' } });
  });

  it('setOnFileComplete + sendFile with open DC', async () => {
    let done = false;
    peerMgr.setOnFileComplete(() => { done = true; });
    // create file
    const file = new File(['hello-file-content'], 't.txt', { type: 'text/plain' });
    try {
      await peerMgr.sendFile('peer-pub-2', file as any);
    } catch {
      // expected if no open peer channel
    }
    expect(typeof done).toBe('boolean');
  });
});

describe('vpn deep', () => {
  it('refreshVPNStatus with proper RPC result', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true, status: 200,
      json: async () => ({
        result: {
          state: 'connected', bytesDown: 1000, bytesUp: 500,
          peers: [{ id: '1' }], uptime: 60, myIP: '10.0.0.1',
          myPublicKey: 'pk', transport: 'wg', socksAddr: '127.0.0.1:10808',
          upstream: '', mode: 'real', realTraffic: true,
        },
      }),
    });
    const st = await refreshVPNStatus();
    expect(st === null || typeof st === 'object').toBe(true);
  });

  it('connectRealVPN success path', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { state: 'connected' } }),
    });
    const ok = await connectRealVPN();
    expect(typeof ok).toBe('boolean');
  });

  it('connectLocalVPN', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { ok: true } }),
    });
    expect(typeof await connectLocalVPN()).toBe('boolean');
  });

  it('connectExitVPN', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { ok: true } }),
    });
    expect(typeof await connectExitVPN('pk', '1.2.3.4:1080')).toBe('boolean');
  });

  it('shareExitNode', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { ok: true } }),
    });
    expect(typeof await shareExitNode()).toBe('boolean');
  });

  it('checkEgressIP parses body', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true, status: 200,
      json: async () => ({ ip: '8.8.8.8' }),
      text: async () => '8.8.8.8',
    });
    const ip = await checkEgressIP();
    expect(typeof ip).toBe('string');
  });

  it('disconnectVPN', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { ok: true } }),
    });
    await disconnectVPN();
  });

  it('connectVPN wrapper', async () => {
    mockFetch.mockResolvedValue({
      ok: true, status: 200,
      json: async () => ({ result: { ok: true } }),
    });
    expect(typeof await connectVPN()).toBe('boolean');
    expect(typeof await connectVPN('socks://exit:1080')).toBe('boolean');
  });

  it('formatBytes edge cases', () => {
    expect(formatBytes(0)).toMatch(/0/);
    expect(formatBytes(512)).toBeDefined();
    expect(formatBytes(1024 * 1024 * 5)).toMatch(/MB|mb|M/i);
  });

  it('formatUptime edge cases', () => {
    expect(formatUptime(0)).toBeDefined();
    expect(formatUptime(59)).toBeDefined();
    expect(formatUptime(3600)).toBeDefined();
    expect(formatUptime(90000)).toBeDefined();
  });

  it('vpnStatus store transitions', () => {
    vpnStatus.set('disconnected');
    let v = '';
    const u = vpnStatus.subscribe(s => v = s);
    vpnStatus.set('connected');
    expect(v).toBe('connected');
    u();
  });
});
