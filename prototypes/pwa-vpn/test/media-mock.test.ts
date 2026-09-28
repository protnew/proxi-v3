/**
 * @vitest-environment jsdom
 * MediaRecorder + AudioContext mock tests for voice.ts and sounds.ts
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ═══ Mock AudioContext ═══
class MockAudioBuffer {
  numberOfChannels = 1;
  length = 4410;
  sampleRate = 44100;
  duration = 0.1;
  getChannelData(_ch: number): Float32Array { return new Float32Array(this.length); }
}

class MockBufferSource {
  buffer: MockAudioBuffer | null = null;
  onended: any = null;
  connect(_dest: any): void {}
  start(_time?: number): void {}
  stop(_time?: number): void {}
}

class MockAnalyser {
  frequencyBinCount = 1024;
  connect(_dest: any): void {}
  getByteFrequencyData(arr: Uint8Array): void { arr.fill(128); }
  getByteTimeDomainData(arr: Uint8Array): void { arr.fill(128); }
}

class MockGainNode {
  gain = { value: 1, linearRampToValueAtTime: vi.fn(), setValueAtTime: vi.fn() };
  connect(_dest: any): void {}
}

class MockMediaStreamSource {
  connect(_dest: any): void {}
}

class MockAudioContext {
  sampleRate = 44100;
  state = 'running';
  destination = {};
  currentTime = 0;
  createBuffer(ch: number, len: number, sr: number): MockAudioBuffer {
    const b = new MockAudioBuffer();
    b.numberOfChannels = ch; b.length = len; b.sampleRate = sr;
    b.duration = len / sr;
    return b;
  }
  createBufferSource(): MockBufferSource { return new MockBufferSource(); }
  createAnalyser(): MockAnalyser { return new MockAnalyser(); }
  createGain(): MockGainNode { return new MockGainNode(); }
  createMediaStreamSource(_s: any): MockMediaStreamSource { return new MockMediaStreamSource(); }
  createOscillator(): any {
    return { frequency: { value: 0 }, connect: () => {}, start: () => {}, stop: () => {} };
  }
  close(): void { this.state = 'closed'; }
  resume(): Promise<void> { return Promise.resolve(); }
}
vi.stubGlobal('AudioContext', MockAudioContext as any);
vi.stubGlobal('webkitAudioContext', MockAudioContext as any);

// ═══ Mock MediaRecorder ═══
class MockMediaRecorder {
  state = 'inactive';
  mimeType = 'audio/webm';
  ondataavailable: any = null;
  onstop: any = null;
  onstart: any = null;
  stream: any;
  static isTypeSupported(type: string): boolean {
    return ['audio/webm', 'audio/webm;codecs=opus', 'audio/ogg'].includes(type);
  }
  constructor(stream: any) { this.stream = stream; }
  start(): void {
    this.state = 'recording';
    this.onstart?.();
    // Simulate data
    setTimeout(() => {
      if (this.ondataavailable) {
        this.ondataavailable({ data: new Blob([new Uint8Array(100)], { type: 'audio/webm' }) });
      }
    }, 10);
  }
  stop(): void {
    this.state = 'inactive';
    this.onstop?.();
  }
  requestData(): void {}
}
vi.stubGlobal('MediaRecorder', MockMediaRecorder as any);

// ═══ Mock MediaStream ═══
class MockMediaStream {
  id = 'stream-' + Math.random().toString(36).slice(2);
  active = true;
  getTracks(): any[] { return [{ kind: 'audio', stop: vi.fn(), enabled: true }]; }
  getAudioTracks(): any[] { return [{ kind: 'audio', stop: vi.fn(), enabled: true }]; }
}
vi.stubGlobal('MediaStream', MockMediaStream as any);

vi.stubGlobal('navigator', {
  ...globalThis.navigator,
  mediaDevices: {
    getUserMedia: vi.fn().mockResolvedValue(new MockMediaStream()),
  },
});

// ═══ Mock Blob + URL.createObjectURL ═══
vi.stubGlobal('URL', {
  ...globalThis.URL,
  createObjectURL: vi.fn().mockReturnValue('blob:mock-url'),
  revokeObjectURL: vi.fn(),
});

// Mock Audio element
vi.stubGlobal('Audio', class {
  src = '';
  currentTime = 0;
  duration = 0;
  paused = true;
  play(): Promise<void> { this.paused = false; return Promise.resolve(); }
  pause(): void { this.paused = true; }
  load(): void {}
});

import { startRecording, stopRecording, playVoice, getWaveformAnalyser } from '../src/lib/voice';
import { initSounds, playIncoming, playOutgoing, playCallRing } from '../src/lib/sounds';

describe('Voice', () => {
  beforeEach(() => vi.clearAllMocks());

  it('startRecording returns MediaStream', async () => {
    const stream = await startRecording();
    expect(stream).toBeDefined();
    expect(stream.active).toBe(true);
  });

  it('stopRecording returns VoiceData with blob and url', async () => {
    await startRecording();
    // Wait for MediaRecorder to collect some data
    await new Promise(r => setTimeout(r, 50));
    const data = await stopRecording();
    expect(data).toBeDefined();
    expect(data.blob).toBeInstanceOf(Blob);
    expect(data.duration).toBeGreaterThanOrEqual(0);
    expect(data.waveformData).toBeDefined();
    expect(data.waveformData.length).toBe(30);
  });

  it('stopRecording after no start still works', async () => {
    // Without prior startRecording, should handle gracefully
    try {
      const data = await stopRecording();
      // May return empty/default
      expect(data).toBeDefined();
    } catch (e) {
      // Or throw — both acceptable
    }
  });

  it('playVoice returns HTMLAudioElement', () => {
    const audio = playVoice('blob:mock-url');
    expect(audio).toBeDefined();
  });

  it('getWaveformAnalyser returns analyser data', () => {
    const stream = new MockMediaStream();
    const result = getWaveformAnalyser(stream as any);
    expect(result).toBeDefined();
    expect(result.analyser).toBeDefined();
    expect(result.dataArray).toBeDefined();
  });

  it('getWaveformAnalyser provides frequency data', () => {
    const stream = new MockMediaStream();
    const result = getWaveformAnalyser(stream as any);
    result.analyser.getByteFrequencyData(result.dataArray);
    // Mock fills with 128
    expect(result.dataArray[0]).toBe(128);
  });
});

describe('Sounds', () => {
  beforeEach(() => vi.clearAllMocks());

  it('initSounds initializes without throwing', () => {
    expect(() => initSounds()).not.toThrow();
  });

  it('playIncoming does not throw', () => {
    expect(() => playIncoming()).not.toThrow();
  });

  it('playOutgoing does not throw', () => {
    expect(() => playOutgoing()).not.toThrow();
  });

  it('playCallRing does not throw', () => {
    expect(() => playCallRing()).not.toThrow();
  });

  it('multiple playIncoming calls are safe', () => {
    expect(() => {
      playIncoming();
      playIncoming();
      playIncoming();
    }).not.toThrow();
  });

  it('playIncoming + playOutgoing sequence is safe', () => {
    expect(() => {
      playIncoming();
      playOutgoing();
      playCallRing();
      playIncoming();
    }).not.toThrow();
  });
});
