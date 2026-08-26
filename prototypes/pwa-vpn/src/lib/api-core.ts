/**
 * API client for Go backend (Indestructible Messenger / Proxi).
 *
 * P0-4 RESCUE 20260720 → INTEGRATION 20260721:
 *   - Real WebSocket connection to /ws (Go chat.hub.ServeWS)
 *   - REST endpoints aligned with Go route registrations
 *   - Message format matches chat.Message struct (chat/message.go)
 *
 * Backend routes (from cmd/webserver/startup.go):
 *   /ws                    — WebSocket (chat)
 *   /api/auth/{signup,login,refresh}
 *   /api/identity          — GET current identity (auto-generates)
 *   /api/online            — GET online users list
 *   /api/messages          — GET history / POST send
 *   /api/profiles          — GET/POST profile
 *   /api/groups/{create,list,members,kick,promote}
 *   /api/search            — GET full-text search
 *   /api/files/{upload,GET /:id}
 *   /api/media/{upload,GET /:id}
 *   /api/reactions, /api/read-receipts
 *
 * Message wire format (chat.Message struct):
 *   { type, from, to, text, ts, id?, replyTo?, voiceData?, voiceDuration? }
 *   type: "chat" | "join" | "leave" | "typing" | "voice" | "key_exchange"
 *   to:    userId for DM, "broadcast" or "" for public
 */

import type { Message } from '../stores/messenger';
import { makeChatPayload } from './api-payload';
import * as secp from '@noble/secp256k1';
import { encryptDM } from './nip-e2e';

// If VITE_API_URL is empty → same-origin (Go serves both dist + API on one port).
// This is how "Telegram Web" works: one URL, no separate API host.
const API_BASE = (import.meta as any).env?.VITE_API_URL || '';
// In browser: derive WS from location. In Node.js tests: fallback to localhost.
function deriveWsBase(): string {
  if (API_BASE) return API_BASE.replace(/^http/, 'ws');
  if (typeof location !== 'undefined') {
    return `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}`;
  }
  return 'ws://localhost:8090'; // Node.js test fallback
}
const WS_BASE = deriveWsBase();

export interface ApiResponse<T = any> {
  data?: T;
  error?: string;
  status: number;
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  try {
    const token = getStoredToken();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const resp = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
      credentials: 'include',
    });

    const data = await resp.json().catch(() => null);
    return { data, status: resp.status, error: resp.ok ? undefined : data?.error || resp.statusText };
  } catch (e: any) {
    return { status: 0, error: e.message };
  }
}

// ============================================================
// Identity & Token storage
// ============================================================

let e2eEnabled = true;
export function isE2EEnabledLocal(): boolean { return e2eEnabled; }
export function setE2EEnabledLocal(v: boolean): void { e2eEnabled = v; }

let cachedIdentity: { pubkey: string; privateKey: string; token?: string; userId?: string } | null = null;

function getStoredToken(): string | undefined {
  return cachedIdentity?.token || (typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_token') || undefined : undefined);
}

function setStoredToken(token: string | undefined) {
  if (token) {
    if (typeof localStorage !== 'undefined') localStorage.setItem('proxi_token', token);
    if (cachedIdentity) cachedIdentity.token = token;
  }
}

/**
 * Initialize identity synchronously (App.svelte calls without await).
 * Returns cached pubkey immediately. Triggers async backend fetch in background
 * that updates cachedIdentity. Call initIdentityAsync() explicitly in onMount
 * if you need to wait for the real identity.
 */
export function initIdentity(): string {
  if (cachedIdentity?.pubkey) {
    exposeProxiDebug();
    return cachedIdentity.pubkey;
  }

  // Demo mode: check for fixed test identities (Alice/Bob)
  const demoRole = typeof localStorage !== 'undefined' ? localStorage.getItem('proxi_demo_role') : null;
  if (demoRole === 'tester1') {
    const k1 = '1'.repeat(64);
    cachedIdentity = { pubkey: k1, privateKey: k1 };
    exposeProxiDebug();
    return cachedIdentity.pubkey;
  }
  if (demoRole === 'tester2') {
    const k2 = '2'.repeat(64);
    cachedIdentity = { pubkey: k2, privateKey: k2 };
    exposeProxiDebug();
    return cachedIdentity.pubkey;
  }

  // SL-020/021/022: Load existing or generate real secp256k1 keys in browser.
  // Keys NEVER leave the browser. Go server is optional (for WS relay only).
  try {
    // VPN-PEER-REAL fix: Load existing key from localStorage BEFORE generating new
    const savedPriv = typeof localStorage !== 'undefined' ? localStorage.getItem('indestructible-seckey') : null;
    const savedPub = typeof localStorage !== 'undefined' ? localStorage.getItem('indestructible-pubkey') : null;
    if (savedPriv && savedPub && savedPriv.length === 64) {
      cachedIdentity = { pubkey: savedPub, privateKey: savedPriv };
      initIdentityAsync();
      exposeProxiDebug();
      return cachedIdentity.pubkey;
    }
    // No saved key — do NOT silent-create (ONB-000 AuthScreen owns first-run)
    // Demo roles handled above. Product path: AuthScreen → createIdentity.
    cachedIdentity = null as any;
    exposeProxiDebug();
    return '';
  } catch (e) {
    console.warn('[api] initIdentity load failed', e);
    cachedIdentity = null as any;
  }

  if (!cachedIdentity?.pubkey) {
    exposeProxiDebug();
    return '';
  }

  initIdentityAsync();
  exposeProxiDebug();

  return cachedIdentity.pubkey;
}

function exposeProxiDebug(): void {
  if (typeof window === 'undefined') return;
  try {
    (window as any).__proxiPubkey = cachedIdentity?.pubkey || '';
    (window as any).__proxiUserId = cachedIdentity?.userId || '';
    (window as any).__proxiGetPubkey = () => cachedIdentity?.pubkey || '';
  } catch { /* ignore */ }
}
/** Async identity initialization — Go signup OPTIONAL (non-blocking). Keys stay local. */
export async function initIdentityAsync(): Promise<string> {
  if (!cachedIdentity?.pubkey) return '';

  // SL-022: Go signup is optional — only for WS relay JWT. Identity is browser-local.
  const signupNpub = cachedIdentity.pubkey;
  console.log('[api] initIdentityAsync: local pubkey=' + signupNpub.slice(0, 16) + '...');

  try {
    const signupResp = await request<{ access_token: string; refresh_token: string; user_id: string }>(
      '/api/auth/signup',
      { method: 'POST', body: JSON.stringify({ npub: signupNpub, username: 'user_' + signupNpub.slice(0, 8) }) }
    );

    if (signupResp.data?.access_token) {
      setStoredToken(signupResp.data.access_token);
      if (cachedIdentity) cachedIdentity.userId = signupResp.data.user_id;
      console.log('[api] initIdentityAsync: Go JWT stored, user_id=' + signupResp.data.user_id);
      exposeProxiDebug();
    } else {
      console.log('[api] initIdentityAsync: Go signup status=' + signupResp.status + ' — local identity only');
    }
  } catch (e) {
    console.log('[api] initIdentityAsync: Go unavailable — decentralized mode (keys stay local)');
  }

  return cachedIdentity.pubkey;
}


// ============================================================
// WebSocket connection (real Go /ws endpoint)
// ============================================================

type MessageCallback = (msg: Message) => void;
type PresenceCallback = (pubkey: string, online: boolean) => void;
type TypingCallback = (pubkey: string) => void;

const messageCallbacks: Set<MessageCallback> = new Set();
const presenceCallbacks: Set<PresenceCallback> = new Set();
const typingCallbacks: Set<TypingCallback> = new Set();

let wsConnection: WebSocket | null = null;
let wsReconnectTimer: ReturnType<typeof setTimeout> | null = null;
let lastStatus: Record<string, any> = { connected: false, relays: 0 };

function buildWsUrl(): string {
  const token = getStoredToken();
  const pubkey = cachedIdentity?.pubkey || '';
  const params = new URLSearchParams();
  if (token) params.set('token', token);
  if (pubkey) params.set('userId', pubkey);
  const qs = params.toString();
  return `${WS_BASE}/ws${qs ? '?' + qs : ''}`;
}

function handleWsMessage(event: MessageEvent) {
  let msg: any;
  try {
    msg = JSON.parse(event.data);
  } catch {
    return;
  }
  if (!msg || typeof msg !== 'object') return;

  // Map Go chat.Message → Svelte Message shape.
  const mapped: Message = {
    id: msg.id || `${msg.from || 'unknown'}-${msg.ts || Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    from: msg.from || '',
    to: msg.to || '',
    text: msg.text || '',
    timestamp: msg.ts || Date.now() / 1000,
    type: mapMsgType(msg.type),
    read: false,
    fileName: undefined,
    fileSize: undefined,
    fileUrl: undefined,
    voiceDuration: msg.voiceDuration,
    replyTo: msg.replyTo,
    forwardedFrom: msg.forwardedFrom,
  };

  switch (msg.type) {
    case 'join':
      presenceCallbacks.forEach(cb => cb(msg.from, true));
      break;
    case 'leave':
      presenceCallbacks.forEach(cb => cb(msg.from, false));
      break;
    case 'typing':
      typingCallbacks.forEach(cb => cb(msg.from));
      break;
    default:
      messageCallbacks.forEach(cb => cb(mapped));
  }
}

function mapMsgType(t?: string): 'text' | 'voice' | 'file' | 'image' | 'system' {
  switch (t) {
    case 'voice': return 'voice';
    case 'file': return 'file';
    case 'image': return 'image';
    case 'join':
    case 'leave':
    case 'system': return 'system';
    default: return 'text';
  }
}

/** Connect to Go backend /ws. Returns relay count (0 or 1). */
export async function connectRelays(): Promise<number> {
  // Ensure identity + token are initialized
  if (!cachedIdentity?.pubkey) {
    await initIdentityAsync();
  }

  // Close existing connection
  if (wsConnection) {
    try { wsConnection.close(); } catch {}
    wsConnection = null;
  }

  const wsToken = getStoredToken();
  console.log('[api] connectRelays: token=' + (wsToken ? wsToken.slice(0, 20) + '...' : 'NONE') + ', pubkey=' + (cachedIdentity?.pubkey || 'NONE').slice(0, 16));

  return new Promise<number>((resolve) => {
    let resolved = false;
    try {
      const ws = new WebSocket(buildWsUrl());
      wsConnection = ws;

      ws.onopen = () => {
        lastStatus = { connected: true, relays: 1, mode: 'go-backend', url: buildWsUrl() };
        if (!resolved) { resolved = true; resolve(1); }

        // Send join broadcast
        sendRaw({ type: 'join', from: cachedIdentity?.pubkey || '', to: 'broadcast', ts: Math.floor(Date.now() / 1000) });

        // Poll online users via REST (returns {count, users:[]})
        request<{ count: number; users: string[] }>('/api/online').then(resp => {
          if (resp.data && resp.data.users) {
            resp.data.users.forEach(pk => presenceCallbacks.forEach(cb => cb(pk, true)));
          }
        });
      };

      ws.onmessage = handleWsMessage;

      ws.onerror = (e) => {
        lastStatus = { connected: false, relays: 0, error: 'ws-error', detail: String(e) };
      };

      ws.onclose = () => {
        lastStatus = { connected: false, relays: 0, closed: true };
        // Auto-reconnect after 3 seconds
        if (wsReconnectTimer) clearTimeout(wsReconnectTimer);
        wsReconnectTimer = setTimeout(() => { connectRelays(); }, 3000);
      };

      // If no open event in 5 seconds, resolve with 0
      setTimeout(() => { if (!resolved) { resolved = true; resolve(0); } }, 5000);
    } catch (e) {
      lastStatus = { connected: false, relays: 0, error: String(e) };
      resolve(0);
    }
  });
}

function sendRaw(obj: any): boolean {
  if (!wsConnection || wsConnection.readyState !== WebSocket.OPEN) return false;
  try {
    wsConnection.send(JSON.stringify(obj));
    return true;
  } catch {
    return false;
  }
}

// ============================================================
// Public API (used by Svelte components)
// ============================================================

/** Broadcast presence (online/offline) to other clients via WS. */
export function sendPresence(online: boolean): void {
  sendRaw({
    type: online ? 'join' : 'leave',
    from: cachedIdentity?.pubkey || '',
    to: 'broadcast',
    ts: Math.floor(Date.now() / 1000),
  });
}

/** Resolve pubkey → display name (from local cache or /api/profiles). */
export function getName(pubkey: string): string {
  if (typeof localStorage !== 'undefined') {
    const cached = localStorage.getItem(`proxi_name_${pubkey}`);
    if (cached) return cached;
  }
  return pubkey.slice(0, 8);
}

export function onMessage(cb: MessageCallback): void { messageCallbacks.add(cb); }
export function onPresence(cb: PresenceCallback): void { presenceCallbacks.add(cb); }
export function onTyping(cb: TypingCallback): void { typingCallbacks.add(cb); }
export function getStatus(): Record<string, any> { return lastStatus; }



/** Set identity directly (used by AuthScreen after key creation/import). */
export function setIdentity(pubkey: string, privateKey: string): void {
  cachedIdentity = { pubkey, privateKey };
}

// ============================================================
// Exports for api-actions.ts (shared state)
// ============================================================

export function getCachedIdentity() {
  return cachedIdentity;
}

export function isE2EEnabled(): boolean {
  return e2eEnabled;
}

export function getWsConnection(): WebSocket | null {
  return wsConnection;
}

// These were local in the monolithic file, now exported for api-actions.ts
export { getStoredToken, sendRaw, API_BASE };
