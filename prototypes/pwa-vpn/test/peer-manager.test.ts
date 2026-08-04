/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from 'vitest';

vi.mock('../src/lib/webrtc-tunnel', () => ({
  WebRTCTunnel: vi.fn().mockImplementation(() => ({
    on: vi.fn(), send: vi.fn(), close: vi.fn(), state: 'disconnected',
  })),
}));

import { setOnFileComplete } from '../src/lib/peer-manager';

describe('peer-manager', () => {
  it('setOnFileComplete registers callback', () => {
    expect(typeof setOnFileComplete).toBe('function');
    expect(() => setOnFileComplete(() => {})).not.toThrow();
  });
});
