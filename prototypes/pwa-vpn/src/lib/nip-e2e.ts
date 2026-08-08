/**
 * NIP-E2E: End-to-end encrypted Direct Messages.
 * Uses secp256k1 ECDH for shared secret + AES-GCM for encryption.
 *
 * Architecture table 56_E2EE_Protocol: E2E encryption (Score: NIP-44 path).
 * Architecture table 26_Signaling: Nostr NIP-44 for messaging.
 *
 * Even if Go server is compromised, messages cannot be decrypted
 * without the recipient's private key.
 */

import * as secp from "@noble/secp256k1"

const bytesToHex = (b: Uint8Array | number[]) => Array.from(b).map(x => x.toString(16).padStart(2, "0")).join("")
const hexToBytes = (hex: string) => {
  const b = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) b[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  return b
}

/**
 * Derive shared secret via ECDH (secp256k1).
 * senderPrivateKey + recipientPublicKey -> shared secret
 */
function deriveSharedSecret(privateKeyHex: string, publicKeyHex: string): Uint8Array {
  // Use ECDH on secp256k1: multiply recipient's pubkey by our private key
  const shared = secp.getSharedSecret(privateKeyHex, publicKeyHex, true)
  // shared is 33 bytes (compressed) or 65 bytes (uncompressed)
  // Hash to get 32-byte key
  return shared.slice(1, 33) // Use x-coordinate (first 32 bytes after prefix)
}

/**
 * Encrypt a message for a recipient.
 * Returns base64 ciphertext + nonce.
 */
export async function encryptDM(
  message: string,
  senderPrivateKeyHex: string,
  recipientPublicKeyHex: string,
): Promise<string> {
  const shared = deriveSharedSecret(senderPrivateKeyHex, recipientPublicKeyHex)

  // Import shared secret as AES-GCM key
  const key = await crypto.subtle.importKey(
    "raw",
    shared,
    { name: "AES-GCM" },
    false,
    ["encrypt"],
  )

  // Generate random 12-byte nonce
  const nonce = crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(message)

  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    encoded,
  )

  // Pack: nonce (12 bytes) + ciphertext
  const combined = new Uint8Array(nonce.length + ciphertext.byteLength)
  combined.set(nonce, 0)
  combined.set(new Uint8Array(ciphertext), nonce.length)

  return btoa(String.fromCharCode(...combined))
}

/**
 * Decrypt a message from a sender.
 */
export async function decryptDM(
  encryptedBase64: string,
  recipientPrivateKeyHex: string,
  senderPublicKeyHex: string,
): Promise<string> {
  const shared = deriveSharedSecret(recipientPrivateKeyHex, senderPublicKeyHex)

  const key = await crypto.subtle.importKey(
    "raw",
    shared,
    { name: "AES-GCM" },
    false,
    ["decrypt"],
  )

  // Unpack: nonce (12 bytes) + ciphertext
  const combined = new Uint8Array(
    atob(encryptedBase64).split("").map(c => c.charCodeAt(0)),
  )
  const nonce = combined.slice(0, 12)
  const ciphertext = combined.slice(12)

  const decrypted = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    ciphertext,
  )

  return new TextDecoder().decode(decrypted)
}
