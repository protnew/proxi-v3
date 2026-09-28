// Универсальный API — работает и в Tauri, и в обычном браузере
// NO import from @tauri-apps — Vite создаёт чанк который крашит браузер

let _invoke = null;

export function initApi() {
  // Tauri expose invoke через window.__TAURI_INTERNALS__
  if (typeof window !== 'undefined' && window.__TAURI_INTERNALS__) {
    _invoke = window.__TAURI_INTERNALS__.invoke;
  }
}

export async function api(method, params = {}) {
  if (_invoke) {
    return _invoke(method, params);
  }
  // Web fallback: JSON-RPC через HTTP
  const res = await fetch('/api/vpn/rpc', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params })
  });
  const data = await res.json();
  if (data.error) throw new Error(data.error.message);
  return data.result;
}

export function isTauri() {
  return !!_invoke;
}
