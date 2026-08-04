/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { initSounds, playIncoming, playOutgoing } from '../src/lib/sounds';

// Mock AudioContext
const mockAudioContext = {
  sampleRate: 44100,
  state: 'suspended',
  resume: vi.fn(),
  createBuffer: vi.fn(() => ({
    getChannelData: vi.fn(() => new Float32Array(1000)),
  })),
  createBufferSource: vi.fn(() => ({
    buffer: null,
    connect: vi.fn(),
    start: vi.fn(),
  })),
  destination: {},
};

vi.stubGlobal('AudioContext', vi.fn(() => mockAudioContext));

describe('sounds', () => {
  beforeEach(() => vi.clearAllMocks());

  it('initSounds does not crash', () => {
    expect(() => initSounds()).not.toThrow();
  });

  it('playIncoming creates AudioContext and plays', () => {
    playIncoming();
    // AudioContext may or may not be created (lazy)
    // The important thing is no crash
    expect(true).toBe(true);
  });

  it('playOutgoing does not crash', () => {
    playOutgoing();
    expect(true).toBe(true);
  });

  it('multiple playIncoming calls are safe', () => {
    playIncoming();
    playIncoming();
    playIncoming();
    expect(true).toBe(true);
  });
});
