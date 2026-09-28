/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi } from 'vitest';
import { stopRecording, playVoice } from '../src/lib/voice';

describe('voice messages', () => {
  it('stopRecording with no active recording returns empty VoiceData', async () => {
    const result = await stopRecording();
    expect(result).toBeDefined();
    expect(result.blob).toBeDefined();
    expect(result.duration).toBe(0);
    expect(result.waveformData).toEqual([]);
  });

  it('playVoice returns Audio element', () => {
    // jsdom provides Audio constructor
    const audio = playVoice('data:audio/webm;base64,AAA');
    expect(audio).toBeDefined();
  });
});
