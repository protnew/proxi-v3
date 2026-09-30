/**
 * TZ-04-20260930 §3.4 — incoming `filemeta:{json}` descriptors (P22 wire).
 * Mirrors the Android contract (AttachmentManager.kt sanitize*): only
 * relative same-origin /api/files/ paths are ever fetched with the JWT;
 * names are reduced to a basename. A hostile descriptor degrades to a plain
 * text bubble (no button, no fetch).
 */

export interface ParsedFilemeta {
  kind: 'file' | 'voice';
  fileName: string;
  fileSize: number;
  fileUrl: string; // absolute, token-appended (safe for <img>/<audio>/<a>)
  mime: string;
  voiceDuration?: number;
}

/** Only a relative /api/files/... path is acceptable (P11 contract). */
export function sanitizeFileUrl(raw: unknown): string | null {
  if (typeof raw !== 'string') return null;
  const u = raw.trim();
  if (!u.startsWith('/api/files/')) return null;
  if (/(@|\.\.|:\/\/|\\)/.test(u)) return null;
  if (!/^\/api\/files\/[A-Za-z0-9._-]+$/.test(u)) return null;
  return u;
}

/** Basename only — no traversal, no separators. */
export function sanitizeFileName(raw: unknown): string {
  if (typeof raw !== 'string') return 'file';
  const base = raw.split(/[\\/]/).pop() || '';
  if (!base || base === '.' || base === '..') return 'file';
  return base.slice(0, 120);
}

/**
 * Parse a decrypted message body. Returns null when the body is not a
 * filemeta descriptor or the descriptor is hostile/invalid.
 * `downloadUrl` builds the authed URL from the sanitized relative path.
 */
export function parseFilemeta(
  text: string,
  downloadUrl: (relPath: string) => string,
): ParsedFilemeta | null {
  if (!text || !text.startsWith('filemeta:')) return null;
  let meta: any;
  try {
    meta = JSON.parse(text.slice('filemeta:'.length));
  } catch {
    return null;
  }
  if (!meta || typeof meta !== 'object') return null;
  const rel = sanitizeFileUrl(meta.url);
  if (!rel) return null;
  return {
    kind: meta.kind === 'voice' ? 'voice' : 'file',
    fileName: sanitizeFileName(meta.name),
    fileSize: Number.isFinite(+meta.size) ? Math.max(0, +meta.size) : 0,
    fileUrl: downloadUrl(rel),
    mime: typeof meta.mime === 'string' ? meta.mime : 'application/octet-stream',
    voiceDuration: Number.isFinite(+meta.dur) ? +meta.dur : undefined,
  };
}
