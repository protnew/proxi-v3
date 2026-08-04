/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from 'vitest';

// Mock WebRTCTunnel as ESM module
vi.mock('../src/lib/webrtc-tunnel', () => ({
  WebRTCTunnel: vi.fn(),
}));

// Mock fetch
vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({
  ok: true, status: 200, text: () => Promise.resolve('OK'),
})));

import { ExitNode } from '../src/lib/exit-node';

describe('ExitNode', () => {
  it('can be instantiated', () => {
    const node = new ExitNode();
    expect(node).toBeDefined();
  });

  it('addTunnel stores tunnel reference without crash', () => {
    const node = new ExitNode();
    const fakeTunnel = { on: vi.fn(), send: vi.fn(), close: vi.fn() };
    expect(() => node.addTunnel('peer123', fakeTunnel as any)).not.toThrow();
  });

  it('removeTunnel removes tunnel', () => {
    const node = new ExitNode();
    const fakeTunnel = { on: vi.fn(), send: vi.fn(), close: vi.fn() };
    node.addTunnel('peer456', fakeTunnel as any);
    expect(() => node.removeTunnel('peer456')).not.toThrow();
  });

  it('removeTunnel for unknown peer does not crash', () => {
    const node = new ExitNode();
    expect(() => node.removeTunnel('unknown')).not.toThrow();
  });
});
