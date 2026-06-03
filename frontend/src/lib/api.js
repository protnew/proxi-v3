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
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };
  if (token) {
    headers['Authorization'] = 'Bearer ' + token;
  }

  const res = await fetch(API_BASE + path, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    clearToken();
    const err = new Error('Unauthorized');
    err.status = 401;
    throw err;
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      message = body.error || body.message || message;
    } catch {}
    const err = new Error(message);
    err.status = res.status;
    throw err;
  }

  return res.json();
}

// ── Auth ──────────────────────────────────────────────────────────

export async function login(npub) {
  const data = await apiFetch('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ npub }),
  });
  if (data.token) setToken(data.token);
  return data;
}

export async function signup(npub, username) {
  const data = await apiFetch('/api/auth/signup', {
    method: 'POST',
    body: JSON.stringify({ npub, username }),
  });
  if (data.token) setToken(data.token);
  return data;
}

// ── Messages ──────────────────────────────────────────────────────

export async function getMessages(channel, limit = 50) {
  const params = new URLSearchParams({ channel, limit: String(limit) });
  return apiFetch('/api/messages?' + params.toString());
}

export async function sendMessage(to, text) {
  return apiFetch('/api/message', {
    method: 'POST',
    body: JSON.stringify({ to, text }),
  });
}

// ── Channels ──────────────────────────────────────────────────────

export async function getChannels() {
  return apiFetch('/api/channels');
}

// ── Peers ─────────────────────────────────────────────────────────

export async function getPeers() {
  return apiFetch('/api/peers');
}

// ── Stories ───────────────────────────────────────────────────────

export async function getStories() {
  return apiFetch('/api/stories');
}

// ── Identity / Profile ────────────────────────────────────────────

export async function getProfile() {
  return apiFetch('/api/identity');
}

// ── WebSocket helper ──────────────────────────────────────────────

export function createWebSocket(path) {
  const wsBase = API_BASE.replace(/^http/, 'ws');
  const token = getToken();
  const separator = path.includes('?') ? '&' : '?';
  const url = wsBase + path + separator + 'token=' + encodeURIComponent(token);
  return new WebSocket(url);
}
