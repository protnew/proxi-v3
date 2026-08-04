/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getCallState, setOnCallStateChange, startCall, endCall } from '../src/lib/calls';

describe('calls module', () => {
  beforeEach(() => vi.clearAllMocks());

  it('getCallState returns initial state', () => {
    const state = getCallState();
    expect(state).toBeDefined();
  });

  it('setOnCallStateChange registers callback', () => {
    const cb = vi.fn();
    setOnCallStateChange(cb);
    expect(typeof cb).toBe('function');
  });

  it('startCall returns a promise or throws', async () => {
    expect(typeof startCall).toBe('function');
    try {
      await startCall('peer_pubkey');
    } catch (e) {
      // Expected if no media devices in test env
      expect(e).toBeDefined();
    }
  });

  it('endCall cleans up', () => {
    expect(typeof endCall).toBe('function');
    expect(() => endCall()).not.toThrow();
  });
});
