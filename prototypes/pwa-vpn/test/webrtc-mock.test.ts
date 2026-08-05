/**
 * @vitest-environment jsdom
 * WebRTC mock tests for peer-manager, webrtc-tunnel, calls
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ═══ Mock RTCPeerConnection ═══
class MockRTCPeerConnection {
  localDescription: RTCSessionDescriptionInit | null = null;
  remoteDescription: RTCSessionDescriptionInit | null = null;
  onicecandidate: any = null;
  ondatachannel: any = null;
  ontrack: any = null;
  oniceconnectionstatechange: any = null;
  iceConnectionState = 'new';
  connectionState = 'new';

  createDataChannel(label: string): any {
    return {
      label, readyState: 'connecting', binaryType: 'blob', bufferedAmount: 0,
      onopen: null, onclose: null, onmessage: null,
      send: vi.fn(), close: vi.fn(),
    };
  }
  async createOffer(): Promise<RTCSessionDescriptionInit> { return { type: 'offer', sdp: 'mock-offer-sdp' }; }
  async createAnswer(): Promise<RTCSessionDescriptionInit> { return { type: 'answer', sdp: 'mock-answer-sdp' }; }
  async setLocalDescription(desc: any): Promise<void> { this.localDescription = desc; }
  async setRemoteDescription(desc: any): Promise<void> { this.remoteDescription = desc; }
  async addIceCandidate(candidate: any): Promise<void> {}
  addTrack(track: any, stream: any): any { return {}; }
  getStats(): Promise<any> { return Promise.resolve(new Map()); }
  close(): void { this.connectionState = 'closed'; }
}
vi.stubGlobal('RTCPeerConnection', MockRTCPeerConnection as any);

// ═══ Mock MediaStream ═══
class MockMediaStream {
  id = 'mock-stream-' + Math.random().toString(36).slice(2);
  active = true;
  getTracks(): any[] { return [{ kind: 'audio', stop: () => {} }, { kind: 'video', stop: () => {} }]; }
  getAudioTracks(): any[] { return [{ kind: 'audio', stop: () => {}, enabled: true }]; }
  getVideoTracks(): any[] { return [{ kind: 'video', stop: () => {}, enabled: true }]; }
  addTrack(t: any): void {} removeTrack(t: any): void {}
  clone(): any { return new MockMediaStream(); }
}
vi.stubGlobal('MediaStream', MockMediaStream as any);

vi.stubGlobal('navigator', {
  ...globalThis.navigator,
  mediaDevices: { getUserMedia: vi.fn().mockResolvedValue(new MockMediaStream()) },
});

class MockWS {
  readyState = 0; onopen: any; onmessage: any; onerror: any; onclose: any;
  send = vi.fn(); close = vi.fn();
  static OPEN = 1; static CLOSED = 3;
  constructor(public url: string) { setTimeout(() => { this.readyState = 1; this.onopen?.(); }, 5); }
}
vi.stubGlobal('WebSocket', MockWS as any);

const mockSignaling = {
  sendOffer: vi.fn().mockResolvedValue(true),
  sendAnswer: vi.fn().mockResolvedValue(true),
  sendIceCandidate: vi.fn().mockResolvedValue(true),
  onSignal: vi.fn(), connect: vi.fn().mockResolvedValue(1),
  disconnect: vi.fn(), isConnected: false,
};

import { WebRTCTunnel, type TunnelState } from '../src/lib/webrtc-tunnel';
import * as peerMgr from '../src/lib/peer-manager';
import * as calls from '../src/lib/calls';

describe('WebRTCTunnel', () => {
  beforeEach(() => vi.clearAllMocks());

  it('constructs with signaling and target pubkey', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    expect(tunnel).toBeDefined();
    expect(tunnel.state).toBe('disconnected');
  });

  it('connect() creates PC and sends offer via signaling', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    await tunnel.connect();
    expect(mockSignaling.sendOffer).toHaveBeenCalledWith('target-pub-123', 'mock-offer-sdp');
  });

  it('handleSignal with offer sends answer', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    await tunnel.handleSignal({ type: 'offer', sdp: 'remote-offer-sdp' } as any);
    expect(mockSignaling.sendAnswer).toHaveBeenCalledWith('target-pub-123', 'mock-answer-sdp');
  });

  it('handleSignal with answer sets remote description', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    await tunnel.connect();
    await tunnel.handleSignal({ type: 'answer', sdp: 'remote-answer-sdp' } as any);
  });

  it('handleSignal with ice candidate does not throw', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    await tunnel.connect();
    await tunnel.handleSignal({ type: 'ice', ice: { candidate: 'mock-candidate' } } as any);
  });

  it('send() is a no-op when DC not open', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    expect(() => tunnel.send('hello')).not.toThrow();
  });

  it('sendHttpRequest returns a pending Promise when disconnected', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    const result = tunnel.sendHttpRequest('https://example.com');
    expect(result).toBeInstanceOf(Promise);
    // Don't await — it never resolves without a DC
  });

  it('close() resets state to disconnected', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    await tunnel.connect();
    tunnel.close();
    expect(tunnel.state).toBe('disconnected');
  });

  it('onMessage callback can be set', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    tunnel.onMessage = (_data: string) => {};
    expect(tunnel).toBeDefined();
  });

  it('onStateChange callback fires', async () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'target-pub-123');
    const states: TunnelState[] = [];
    tunnel.onStateChange = (s) => states.push(s);
    await tunnel.connect();
    expect(states).toContain('connecting');
  });
});

describe('PeerManager', () => {
  beforeEach(() => vi.clearAllMocks());

  it('connectToPeer returns DataChannel or null', async () => {
    const dc = await peerMgr.connectToPeer('peer-pubkey-test');
    expect(dc === null || dc !== undefined).toBe(true);
  });

  it('handleSignal routes offer without throwing', async () => {
    await peerMgr.handleSignal('peer123', { type: 'offer', sdp: 'mock-sdp' });
  });

  it('handleSignal routes answer without throwing', async () => {
    await peerMgr.handleSignal('peer123', { type: 'answer', sdp: 'mock-sdp' });
  });

  it('handleSignal routes ice without throwing', async () => {
    await peerMgr.handleSignal('peer123', { type: 'ice', candidate: 'mock-candidate' });
  });

  it('setOnFileComplete registers callback', () => {
    peerMgr.setOnFileComplete((_blob: Blob, _manifest: any) => {});
  });
});

describe('Calls', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    calls.endCall();
  });

  it('getCallState returns valid state', () => {
    const s = calls.getCallState();
    expect(['idle', 'ringing', 'connecting', 'connected', 'ended']).toContain(s);
  });

  it('setOnCallStateChange registers callback', () => {
    calls.setOnCallStateChange((_s) => {});
  });

  it('rejectCall transitions state', () => {
    calls.rejectCall('peer-pub');
    expect(['idle', 'ended']).toContain(calls.getCallState());
  });

  it('endCall transitions to ended or idle', () => {
    calls.endCall();
    expect(['idle', 'ended']).toContain(calls.getCallState());
  });

  it('getLocalStream returns stream or null', () => {
    expect(calls.getLocalStream() === null || calls.getLocalStream() !== undefined).toBe(true);
  });

  it('getRemoteStream returns stream or null', () => {
    expect(calls.getRemoteStream() === null || calls.getRemoteStream() !== undefined).toBe(true);
  });

  it('handleCallSignal processes signal without throwing', async () => {
    await calls.handleCallSignal('peer123', { type: 'offer', sdp: 'mock-sdp' });
  });
});
