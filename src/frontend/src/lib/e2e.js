// E2E encryption module
// Uses WebCrypto API: ECDH key exchange + AES-256-GCM
// Key rotation: every 50 messages or 24 hours

let myKeyPair = null;
let myPublicKeyRaw = null;
let sharedKeys = {};
let msgCount = {};
let lastRotation = Date.now();
const ROTATION_INTERVAL = 50;
const ROTATION_TIMEOUT = 24 * 3600 * 1000;
let onKeyRotation = null;

export async function init(keyRotationCallback) {
  onKeyRotation = keyRotationCallback || null;
  await generateKeyPair();
  return getPublicKeyBase64();
}

async function generateKeyPair() {
  myKeyPair = await crypto.subtle.generateKey(
    { name: 'ECDH', namedCurve: 'P-256' },
    true,
    ['deriveKey']
  );
  myPublicKeyRaw = await crypto.subtle.exportKey('raw', myKeyPair.publicKey);
  lastRotation = Date.now();
  msgCount = {};
}

export function getPublicKeyBase64() {
  if (!myPublicKeyRaw) return null;
  return btoa(String.fromCharCode(...new Uint8Array(myPublicKeyRaw)));
}

export async function deriveSharedKey(peerId, peerPublicKeyBase64) {
  const peerRaw = Uint8Array.from(atob(peerPublicKeyBase64), (c) => c.charCodeAt(0));
  const peerPublicKey = await crypto.subtle.importKey(
    'raw',
    peerRaw,
    { name: 'ECDH', namedCurve: 'P-256' },
    false,
    ['deriveKey']
  );

  const sharedKey = await crypto.subtle.deriveKey(
    { name: 'ECDH', public: peerPublicKey },
    myKeyPair.privateKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );

  sharedKeys[peerId] = sharedKey;
  msgCount[peerId] = 0;
  return sharedKey;
}

export async function checkRotation(ws) {
  const totalMsgs = Object.values(msgCount).reduce((a, b) => a + b, 0);
  const timeExpired = Date.now() - lastRotation > ROTATION_TIMEOUT;

  if (totalMsgs >= ROTATION_INTERVAL || timeExpired) {
    await generateKeyPair();
    sharedKeys = {};

    if (ws && ws.readyState === WebSocket.OPEN && onKeyRotation) {
      onKeyRotation(getPublicKeyBase64());
    }
    return true;
  }
  return false;
}

function incrementCount(peerId) {
  msgCount[peerId] = (msgCount[peerId] || 0) + 1;
}

export async function encrypt(peerId, plaintext) {
  const key = sharedKeys[peerId];
  if (!key) return plaintext;

  incrementCount(peerId);

  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encoded = new TextEncoder().encode(plaintext);
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoded);

  const packed = new Uint8Array(iv.length + ciphertext.byteLength);
  packed.set(iv, 0);
  packed.set(new Uint8Array(ciphertext), iv.length);

  return btoa(String.fromCharCode(...packed));
}

export async function decrypt(peerId, encryptedBase64) {
  const key = sharedKeys[peerId];
  if (!key) return encryptedBase64;

  try {
    const packed = Uint8Array.from(atob(encryptedBase64), (c) => c.charCodeAt(0));
    const iv = packed.slice(0, 12);
    const ciphertext = packed.slice(12);

    const decrypted = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ciphertext);
    incrementCount(peerId);
    return new TextDecoder().decode(decrypted);
  } catch (e) {
    return encryptedBase64;
  }
}

export function hasSharedKey(peerId) {
  return !!sharedKeys[peerId];
}

export function getStats() {
  return {
    peers: Object.keys(sharedKeys).length,
    totalMessages: Object.values(msgCount).reduce((a, b) => a + b, 0),
    lastRotation: new Date(lastRotation).toISOString(),
    nextRotation: ROTATION_INTERVAL - Object.values(msgCount).reduce((a, b) => a + b, 0),
  };
}
