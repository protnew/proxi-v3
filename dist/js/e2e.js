// E2E encryption module for Proxi Messenger
// Uses WebCrypto API: ECDH key exchange + AES-256-GCM

const E2E = (() => {
  let myKeyPair = null;       // { publicKey: CryptoKey, privateKey: CryptoKey }
  let myPublicKeyRaw = null;  // ArrayBuffer (raw bytes for exchange)
  let sharedKeys = {};        // peerId → AES CryptoKey (derived shared secret)

  // Initialize: generate ECDH key pair
  async function init() {
    myKeyPair = await crypto.subtle.generateKey(
      { name: 'ECDH', namedCurve: 'P-256' },
      true, // extractable
      ['deriveKey']
    );
    myPublicKeyRaw = await crypto.subtle.exportKey('raw', myKeyPair.publicKey);
    return getPublicKeyBase64();
  }

  // Get public key as base64 string for sending over WS
  function getPublicKeyBase64() {
    if (!myPublicKeyRaw) return null;
    return btoa(String.fromCharCode(...new Uint8Array(myPublicKeyRaw)));
  }

  // Derive shared AES-256-GCM key from peer's public key
  async function deriveSharedKey(peerId, peerPublicKeyBase64) {
    const peerRaw = Uint8Array.from(atob(peerPublicKeyBase64), c => c.charCodeAt(0));
    const peerPublicKey = await crypto.subtle.importKey(
      'raw', peerRaw,
      { name: 'ECDH', namedCurve: 'P-256' },
      false, ['deriveKey']
    );

    const sharedKey = await crypto.subtle.deriveKey(
      { name: 'ECDH', public: peerPublicKey },
      myKeyPair.privateKey,
      { name: 'AES-GCM', length: 256 },
      false,
      ['encrypt', 'decrypt']
    );

    sharedKeys[peerId] = sharedKey;
    return sharedKey;
  }

  // Encrypt message for a peer
  async function encrypt(peerId, plaintext) {
    const key = sharedKeys[peerId];
    if (!key) return plaintext; // fallback: unencrypted

    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encoded = new TextEncoder().encode(plaintext);
    const ciphertext = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv },
      key,
      encoded
    );

    // Pack: iv(12 bytes) + ciphertext
    const packed = new Uint8Array(iv.length + ciphertext.byteLength);
    packed.set(iv, 0);
    packed.set(new Uint8Array(ciphertext), iv.length);

    return btoa(String.fromCharCode(...packed));
  }

  // Decrypt message from a peer
  async function decrypt(peerId, encryptedBase64) {
    const key = sharedKeys[peerId];
    if (!key) return encryptedBase64; // fallback: treat as plaintext

    try {
      const packed = Uint8Array.from(atob(encryptedBase64), c => c.charCodeAt(0));
      const iv = packed.slice(0, 12);
      const ciphertext = packed.slice(12);

      const decrypted = await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv },
        key,
        ciphertext
      );

      return new TextDecoder().decode(decrypted);
    } catch (e) {
      // Decryption failed — return raw
      return encryptedBase64;
    }
  }

  // Check if we have shared key with peer
  function hasSharedKey(peerId) {
    return !!sharedKeys[peerId];
  }

  return { init, getPublicKeyBase64, deriveSharedKey, encrypt, decrypt, hasSharedKey };
})();
