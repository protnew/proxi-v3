/**
 * Global test setup — crypto.subtle + localStorage + IndexedDB for Node
 */
import { webcrypto } from 'crypto';
import 'fake-indexeddb/auto';

if (!globalThis.crypto) {
  (globalThis as any).crypto = webcrypto;
}
// jsdom / partial polyfills may lack subtle
if (!(globalThis as any).crypto?.subtle) {
  (globalThis as any).crypto = webcrypto;
}

// Minimal localStorage polyfill for identity/settings/toast tests
if (typeof (globalThis as any).localStorage === 'undefined') {
  const store = new Map<string, string>();
  (globalThis as any).localStorage = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => { store.set(k, String(v)); },
    removeItem: (k: string) => { store.delete(k); },
    clear: () => { store.clear(); },
    key: (i: number) => Array.from(store.keys())[i] ?? null,
    get length() { return store.size; },
  };
}
