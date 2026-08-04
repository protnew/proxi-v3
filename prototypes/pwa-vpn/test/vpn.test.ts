/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { vpnStatus, vpnStats, refreshVPNStatus, formatBytes, formatUptime } from '../src/lib/vpn';

describe('VPN module', () => {
  it('vpnStatus store has initial state', () => {
    const s = get(vpnStatus);
    expect(typeof s).toBe('string');
    expect(s).toBe('disconnected');
  });

  it('vpnStats store has initial values', () => {
    const s = get(vpnStats);
    expect(s).toBeDefined();
    expect(s.bytesIn).toBe(0);
    expect(s.bytesOut).toBe(0);
    expect(s.peers).toBe(0);
  });

  it('formatBytes converts correctly', () => {
    expect(formatBytes(100)).toBe('100 B');
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1048576)).toBe('1.0 MB');
    expect(formatBytes(1073741824)).toBe('1.0 GB');
  });

  it('formatUptime converts correctly', () => {
    expect(formatUptime(0)).toBe('0:00');
    expect(formatUptime(65)).toBe('1:05');
    expect(formatUptime(3661)).toBe('1:01:01');
  });

  it('refreshVPNStatus is a function', () => {
    expect(typeof refreshVPNStatus).toBe('function');
  });
});
