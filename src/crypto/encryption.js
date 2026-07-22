import nacl from 'tweetnacl';
import util from 'tweetnacl-util';

export function encryptMessage(message, key) {
  const nonce = nacl.randomBytes(nacl.secretbox.nonceLength);
  const messageUint8 = util.decodeUTF8(message);
  const box = nacl.secretbox(messageUint8, nonce, key);
  
  const fullMessage = new Uint8Array(nonce.length + box.length);
  fullMessage.set(nonce);
  fullMessage.set(box, nonce.length);
  
  return util.encodeBase64(fullMessage);
}

export function decryptMessage(encryptedBase64, key) {
  const messageWithNonceAsUint8Array = util.decodeBase64(encryptedBase64);
  const nonce = messageWithNonceAsUint8Array.slice(0, nacl.secretbox.nonceLength);
  const message = messageWithNonceAsUint8Array.slice(
    nacl.secretbox.nonceLength,
    messageWithNonceAsUint8Array.length
  );
  
  const decrypted = nacl.secretbox.open(message, nonce, key);
  
  if (!decrypted) {
    throw new Error('Could not decrypt message');
  }
  
  return util.encodeUTF8(decrypted);
}
