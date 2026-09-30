/**
 * @vitest-environment jsdom
 * TZ-04-20260930 §3.4 — filemeta parser/sanitizer unit tests (P11 contract).
 */
import { describe, it, expect } from 'vitest';
import { parseFilemeta, sanitizeFileUrl, sanitizeFileName } from '../src/lib/filemeta';

const dl = (rel: string) => `http://x/api-files${rel}?token=T`;

describe('sanitizeFileUrl', () => {
  it('accepts a plain relative file path', () => {
    expect(sanitizeFileUrl('/api/files/abc123.jpg')).toBe('/api/files/abc123.jpg');
  });
  it('rejects userinfo-host exfiltration (@)', () => {
    expect(sanitizeFileUrl('@evil.com/x')).toBeNull();
    expect(sanitizeFileUrl('/api/files/x@evil.com')).toBeNull();
  });
  it('rejects schemes and traversal', () => {
    expect(sanitizeFileUrl('https://evil.com/f')).toBeNull();
    expect(sanitizeFileUrl('/api/files/../../etc/passwd')).toBeNull();
    expect(sanitizeFileUrl('/api/files/a\\b')).toBeNull();
  });
  it('rejects non-filemeta paths and junk', () => {
    expect(sanitizeFileUrl('/api/admin')).toBeNull();
    expect(sanitizeFileUrl('')).toBeNull();
    expect(sanitizeFileUrl(42 as any)).toBeNull();
  });
});

describe('sanitizeFileName', () => {
  it('reduces to basename', () => {
    expect(sanitizeFileName('C:\\temp\\..\\report.pdf')).toBe('report.pdf');
    expect(sanitizeFileName('/a/b/c.png')).toBe('c.png');
  });
  it('falls back to file for empty/dot/double-dot', () => {
    expect(sanitizeFileName('')).toBe('file');
    expect(sanitizeFileName('..')).toBe('file');
    expect(sanitizeFileName(undefined as any)).toBe('file');
  });
});

describe('parseFilemeta', () => {
  it('turns a valid descriptor into a file bubble payload', () => {
    const r = parseFilemeta('filemeta:{"url":"/api/files/f1.png","name":"cat.png","size":123,"mime":"image/png","kind":"file"}', dl);
    expect(r).not.toBeNull();
    expect(r!.kind).toBe('file');
    expect(r!.fileName).toBe('cat.png');
    expect(r!.fileUrl).toContain('token=T');
  });
  it('maps voice kind with duration', () => {
    const r = parseFilemeta('filemeta:{"url":"/api/files/v1.bin","name":"voice.webm","size":9,"mime":"audio/webm","kind":"voice","dur":7}', dl);
    expect(r!.kind).toBe('voice');
    expect(r!.voiceDuration).toBe(7);
  });
  it('degrades hostile descriptors to null (plain text bubble)', () => {
    expect(parseFilemeta('filemeta:{"url":"@evil.com/x","name":"x"}', dl)).toBeNull();
    expect(parseFilemeta('filemeta:{broken json', dl)).toBeNull();
    expect(parseFilemeta('hello world', dl)).toBeNull();
    expect(parseFilemeta('', dl)).toBeNull();
  });
});
