/**
 * Global test setup — ensures crypto.subtle is available in Node test env
 */
import { webcrypto } from 'crypto';

if (!globalThis.crypto) {
  (globalThis as any).crypto = webcrypto;
}
