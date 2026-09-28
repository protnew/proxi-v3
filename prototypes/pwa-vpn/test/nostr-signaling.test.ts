/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest';
import {
  KIND_VPN_REQUEST, KIND_VPN_OFFER, KIND_VPN_ANSWER, KIND_VPN_ICE,
} from '../src/lib/nostr-signaling';

describe('Nostr Signaling constants', () => {
  it('VPN kind constants are numbers', () => {
    expect(typeof KIND_VPN_REQUEST).toBe('number');
    expect(typeof KIND_VPN_OFFER).toBe('number');
    expect(typeof KIND_VPN_ANSWER).toBe('number');
    expect(typeof KIND_VPN_ICE).toBe('number');
  });

  it('VPN kinds are in custom range (>30000)', () => {
    expect(KIND_VPN_REQUEST).toBeGreaterThan(30000);
    expect(KIND_VPN_OFFER).toBeGreaterThan(30000);
  });
});
