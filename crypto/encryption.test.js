import { describe, it, expect } from 'vitest';
import { encryptMessage, decryptMessage } from './encryption.js';

describe('XChaCha20-Poly1305 Encryption', () => {
  it('should encrypt and decrypt a message correctly', () => {
    const key = new Uint8Array(32); // mock 32-byte key
    crypto.getRandomValues(key);
    
    const message = "Secret Hello World!";
    const encrypted = encryptMessage(message, key);
    
    expect(encrypted).not.toBe(message);
    
    const decrypted = decryptMessage(encrypted, key);
    expect(decrypted).toBe(message);
  });
});
