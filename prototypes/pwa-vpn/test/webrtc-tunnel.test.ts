/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from 'vitest';

// Mock RTCPeerConnection
vi.stubGlobal('RTCPeerConnection', vi.fn(() => ({
  createDataChannel: vi.fn(() => ({ onopen: null, onclose: null, onmessage: null, send: vi.fn(), close: vi.fn(), readyState: 'connecting' })),
  createOffer: vi.fn(() => Promise.resolve({ type: 'offer', sdp: 'mock' })),
  createAnswer: vi.fn(() => Promise.resolve({ type: 'answer', sdp: 'mock' })),
  setLocalDescription: vi.fn(() => Promise.resolve()),
  setRemoteDescription: vi.fn(() => Promise.resolve()),
  addIceCandidate: vi.fn(() => Promise.resolve()),
  close: vi.fn(),
  onicecandidate: null,
  onconnectionstatechange: null,
})));

// Mock signaling
const mockSignaling = {
  sendOffer: vi.fn(),
  sendAnswer: vi.fn(),
  sendIceCandidate: vi.fn(),
  onOffer: vi.fn(),
  onAnswer: vi.fn(),
  onIce: vi.fn(),
};

import { WebRTCTunnel } from '../src/lib/webrtc-tunnel';

describe('WebRTCTunnel', () => {
  it('can be instantiated with signaling + target', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'peer_pubkey_123');
    expect(tunnel).toBeDefined();
  });

  it('has state getter returning TunnelState string', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'peer_pubkey');
    expect(typeof tunnel.state).toBe('string');
    expect(tunnel.state).toBe('disconnected');
  });

  it('on registers event listener', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'peer_pubkey');
    tunnel.on('message', () => {});
    tunnel.on('stateChange', () => {});
    expect(tunnel).toBeDefined();
  });

  it('send does not crash when disconnected', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'peer_pubkey');
    expect(() => tunnel.send('test')).not.toThrow();
  });

  it('close sets state to disconnected', () => {
    const tunnel = new WebRTCTunnel(mockSignaling as any, 'peer_pubkey');
    tunnel.close();
    expect(tunnel.state).toBe('disconnected');
  });
});
