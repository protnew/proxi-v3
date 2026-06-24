// Proxi Messenger REST API client
// Configurable via VITE_API_URL env var (default http://localhost:9999)

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:9999';

const TOKEN_KEY = 'proxi_jwt';

// ── Token management ──────────────────────────────────────────────

export function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY) || '';
  } catch {
    return '';
  }
}

export function setToken(t) {
  try {
    localStorage.setItem(TOKEN_KEY, t);
  } catch {}
}

export function clearToken() {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {}
}

// ── Generic fetch wrapper ─────────────────────────────────────────

export async function apiFetch(path, options = {}) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000); // 10s timeout
  options.signal = controller.signal;
  const token = getToken();
  
  const headers = new Headers(options.headers || {});
  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  options.headers = headers;

  try {
    const res = await fetch(`${API_BASE}${path}`, options);
    clearTimeout(timeoutId);
    
    if (!res.ok) {
      let errMessage = res.statusText;
      try {
        const errData = await res.json();
        errMessage = errData.error || errData.message || errMessage;
      } catch (e) {}
      throw new Error(errMessage);
    }
    
    // Some endpoints (like DELETE) return 204 No Content
    if (res.status === 204) return null;
    return await res.json();
  } catch (error) {
    clearTimeout(timeoutId);
    console.error(`API Error on ${path}:`, error);
    throw error;
  }
}

// ── Authentication ────────────────────────────────────────────────

export async function signup(passphrase) {
  const data = await apiFetch('/api/auth/signup', {
    method: 'POST',
    body: JSON.stringify({ passphrase }),
  });
  if (data.token) setToken(data.token);
  return data;
}

export async function login(npub, passphrase) {
  const data = await apiFetch('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ npub, passphrase }),
  });
  if (data.token) setToken(data.token);
  return data;
}

export async function getIdentity() {
  return apiFetch('/api/identity');
}

// ── Messages ──────────────────────────────────────────────────────

export async function getMessages(limit = 50) {
  const params = new URLSearchParams({ limit: String(limit) });
  return apiFetch(`/api/messages?${params.toString()}`);
}

export async function sendMessage(to, text) {
  if (!text || text.trim().length === 0) {
    throw new Error('Message text cannot be empty');
  }
  return apiFetch('/api/messages', {
    method: 'POST',
    body: JSON.stringify({ to, text }),
  });
}

// ── Contacts & Profiles ───────────────────────────────────────────

export async function getContacts() {
  return apiFetch('/api/contacts');
}

export async function addContact(npub, name) {
  return apiFetch('/api/contacts', {
    method: 'POST',
    body: JSON.stringify({ npub, name }),
  });
}

export async function getProfiles(npubs) {
  const params = new URLSearchParams({ npubs: npubs.join(',') });
  return apiFetch(`/api/profiles?${params.toString()}`);
}

// ── Channels & Groups ─────────────────────────────────────────────

export async function getChannels() {
  return apiFetch('/api/channels');
}

export async function createChannel(name, description = '') {
  return apiFetch('/api/channels', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  });
}

export async function getGroups() {
  return apiFetch('/api/groups/list');
}

export async function createGroup(name, members) {
  return apiFetch('/api/groups/create', {
    method: 'POST',
    body: JSON.stringify({ name, members }),
  });
}

// ── Advanced ──────────────────────────────────────────────────────

export async function setupSwitch(recipient, message_text, interval_days = 7) {
  return apiFetch('/api/switch/setup', {
    method: 'POST',
    body: JSON.stringify({ recipient, message_text, interval_days }),
  });
}

export async function checkInSwitch() {
  return apiFetch('/api/switch/check-in', {
    method: 'POST'
  });
}

// ── WebSocket helper ──────────────────────────────────────────────

export function createWebSocket(path = '/ws') {
  const wsBase = API_BASE.replace(/^http/, 'ws');
  // Pass auth token via query params or wait for connect to send auth packet
  return new WebSocket(`${wsBase}${path}`);
}
