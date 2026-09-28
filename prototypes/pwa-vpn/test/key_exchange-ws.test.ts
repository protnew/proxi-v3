/**
 * X3/P5 — key_exchange + call signals route before chat default.
 * Lives under test/ (vitest include pattern test slash-star-star slash-star.test.ts).
 */
import { describe, it, expect } from 'vitest';
import { routeWsPayload } from '../src/lib/api-core';

describe('routeWsPayload (X3 hub override)', () => {
  it('routes key_exchange to call', () => {
    expect(routeWsPayload({ type: 'key_exchange', from: 'pk' })).toBe('call');
  });

  it('routes call-offer/answer/ice/hangup/end/reject to call', () => {
    for (const type of ['call-offer', 'call-answer', 'call-ice', 'call-hangup', 'call-end', 'call-reject']) {
      expect(routeWsPayload({ type, from: 'pk' })).toBe('call');
    }
  });

  it('keeps join/leave/typing intact', () => {
    expect(routeWsPayload({ type: 'join', from: 'a' })).toBe('presence-join');
    expect(routeWsPayload({ type: 'leave', from: 'a' })).toBe('presence-leave');
    expect(routeWsPayload({ type: 'typing', from: 'a' })).toBe('typing');
  });

  it('defaults chat/voice to message', () => {
    expect(routeWsPayload({ type: 'chat', text: 'hi' })).toBe('message');
    expect(routeWsPayload({ type: 'voice' })).toBe('message');
    expect(routeWsPayload(null)).toBe('message');
  });
});
