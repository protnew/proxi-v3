/**
 * REST API wrappers and direct function exports.
 * Split from api.ts to stay under 500 LOC limit.
 *
 * Imports shared state (cachedIdentity, wsConnection, etc.) from api-core.ts
 */

import type { Message } from '../stores/messenger';
import { makeChatPayload } from './api-payload';
import { encryptDM } from './nip-e2e';
import {
  request,
  sendRaw,
  getCachedIdentity,
  isE2EEnabled,
  API_BASE,
  getWsConnection,
  type ApiResponse,
} from './api-core';

// ============================================================
// REST API wrappers (used by ChatView, DemoPanel, etc.)
// ============================================================

export function connectWebSocket(token: string, onMessage: (msg: any) => void): WebSocket {
  const WS_BASE = (import.meta as any).env?.VITE_API_URL?.replace(/^http/, 'ws') || 
    (typeof location !== 'undefined' ? `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}` : 'ws://localhost:8090');
  const wsUrl = `${WS_BASE}/ws`;
  const ws = token ? new WebSocket(wsUrl, ['access_token.' + token]) : new WebSocket(wsUrl);
  ws.onmessage = (event) => {
    try { onMessage(JSON.parse(event.data)); } catch { onMessage({ type: 'raw', data: event.data }); }
  };
  return ws;
}

export function buildChatPayload(to: string, content: string) {
  const ident = getCachedIdentity();
  const from = ident?.pubkey || ident?.userId || '';
  return makeChatPayload(from, to, content, isE2EEnabled());
}

export const chatApi = {
  sendDM: async (to: string, content: string) => {
    const ident = getCachedIdentity();
    const from = ident?.pubkey || ident?.userId || '';
    const payload = makeChatPayload(from, to, content, isE2EEnabled());
    const sent = sendRaw(payload);
    if (sent) {
      // FIX double-save: Go hub persists WS chat messages itself (ws_handlers.go SaveMessage).
      // The extra REST POST /api/messages caused every DM to be stored twice.
      return { status: 200, data: { ok: true, via: 'ws', encrypted: payload.encrypted } };
    }
    try {
      if (ident?.privateKey && to.length >= 64) {
        const encrypted = await encryptDM(content, ident.privateKey, to);
        console.log('[api] sendDM: encrypted via NIP-E2E, sending through Nostr...');
        const nostrWs = typeof window !== 'undefined' ? (window as any).__nostrWS : null;
        if (nostrWs && nostrWs.readyState === 1) {
          const createdAt = Math.floor(Date.now() / 1000);
          const eventContent = JSON.stringify({ encrypted, from });
          nostrWs.send(JSON.stringify([
            'EVENT',
            { kind: 4, content: eventContent, created_at: createdAt, tags: [['p', to]], pubkey: from }
          ]));
          console.log('[api] sendDM: sent encrypted Nostr DM');
          return { status: 200, data: { ok: true, via: 'nostr-e2e', encrypted: true } };
        }
      }
    } catch (e) {
      console.warn('[api] sendDM: Nostr E2E failed:', e);
    }
    try {
      return await request('/api/messages', { method: 'POST', body: JSON.stringify({ to, text: content }) });
    } catch {
      console.warn('[api] sendDM: all transports failed, optimistic UI only');
      return { status: 200, data: { ok: true, via: 'optimistic', encrypted: payload.encrypted } };
    }
  },
  sendTyping: (to: string) => {
    sendRaw({ type: 'typing', from: getCachedIdentity()?.pubkey || '', to, ts: Math.floor(Date.now() / 1000) });
    return Promise.resolve({ status: 200 });
  },
  sendGroupMessage: (groupId: string, content: string) => {
    const sent = sendRaw({ type: 'chat', from: getCachedIdentity()?.pubkey || '', to: groupId, text: content, ts: Math.floor(Date.now() / 1000) });
    if (sent) return Promise.resolve({ status: 200, data: { ok: true } });
    return request('/api/messages', { method: 'POST', body: JSON.stringify({ to: groupId, text: content }) });
  },
  getOnline: () => request<{ count: number; users: string[] }>('/api/online'),
  getHistory: (peerId: string, limit = 50) =>
    request<{ count: number; messages: any[] }>(`/api/messages?peer=${encodeURIComponent(peerId)}&limit=${limit}`),
};

export const authApi = {
  login: (pubkey: string, signature: string) =>
    request('/api/auth/login', { method: 'POST', body: JSON.stringify({ pubkey, signature }) }),
  me: () => request('/api/identity'),
};

export const contentApi = {
  list: () => request('/api/content/list'),
  upload: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return request<{ id: string; name: string; size: number; url: string }>('/api/files/upload', { method: 'POST', body: formData });
  },
  download: (id: string) => `${API_BASE}/api/files/${id}`,
  uploadContent: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return request('/api/content/upload', { method: 'POST', body: formData });
  },
  listContent: () => request('/api/content/list'),
};

export const socialApi = {
  getFeed: (limit = 20) => request(`/api/search?q=*&limit=${limit}`),
  publish: (content: string) =>
    request('/api/messages', { method: 'POST', body: JSON.stringify({ to: 'broadcast', text: content }) }),
  getProfile: (pubkey: string) => request(`/api/profiles?pubkey=${encodeURIComponent(pubkey)}`),
};

// ============================================================
// Direct function exports (imported by name in components)
// ============================================================

export async function sendDM(to: string, content: string): Promise<ApiResponse> {
  return chatApi.sendDM(to, content);
}

export async function sendTyping(to: string): Promise<ApiResponse> {
  return chatApi.sendTyping(to);
}

export async function sendGroupMessage(groupId: string, content: string): Promise<ApiResponse> {
  return chatApi.sendGroupMessage(groupId, content);
}

export async function sendBinaryVoice(to: string, blob: Blob): Promise<ApiResponse> {
  const ws = getWsConnection();
  if (ws && ws.readyState === WebSocket.OPEN) {
    try {
      const buf = await blob.arrayBuffer();
      const prefix = new Uint8Array([0x02]);
      const combined = new Uint8Array(prefix.length + buf.byteLength);
      combined.set(prefix, 0);
      combined.set(new Uint8Array(buf), prefix.length);
      ws.send(combined);
      return { status: 200, data: { ok: true, via: 'ws-binary' } };
    } catch (e: any) {
      return { status: 0, error: e.message };
    }
  }
  const formData = new FormData();
  formData.append('file', blob, 'voice.webm');
  formData.append('to', to);
  return request('/api/files/upload', { method: 'POST', body: formData });
}

export async function sendFileManifest(to: string, manifest: Record<string, any>): Promise<ApiResponse> {
  return request('/api/files/upload', {
    method: 'POST',
    body: JSON.stringify({ to, manifest }),
  });
}

export async function createGroup(name: string, members: string[] = []): Promise<ApiResponse> {
  const creatorNpub = getCachedIdentity()?.pubkey || '';
  return request('/api/groups/create', {
    method: 'POST',
    body: JSON.stringify({ name, creatorNpub, members }),
  });
}

export async function listGroups(): Promise<ApiResponse> {
  return request<{ count: number; groups: any[] }>('/api/groups/list');
}

export async function subscribeGroup(groupId: string): Promise<ApiResponse> {
  return request(`/api/channels/subscribe`, {
    method: 'POST',
    body: JSON.stringify({ channelId: groupId }),
  });
}

export async function updateProfile(profile: Record<string, any>): Promise<ApiResponse> {
  const ident = getCachedIdentity();
  if (profile.name && ident?.pubkey) {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(`proxi_name_${ident.pubkey}`, profile.name);
    }
  }
  return request('/api/profiles', {
    method: 'POST',
    body: JSON.stringify(profile),
  });
}

export function getPubkey(): string {
  return getCachedIdentity()?.pubkey || '';
}

export function getUserId(): string {
  return getCachedIdentity()?.userId || '';
}

export function getSeckey(): string {
  return getCachedIdentity()?.privateKey || '';
}

export async function sendCallSignal(to: string, signal: Record<string, any>): Promise<ApiResponse> {
  const sent = sendRaw({
    type: 'key_exchange',
    from: getCachedIdentity()?.pubkey || '',
    to,
    publicKey: JSON.stringify(signal),
    ts: Math.floor(Date.now() / 1000),
  });
  if (sent) return { status: 200, data: { ok: true } };
  return { status: 0, error: 'WS not connected' };
}
