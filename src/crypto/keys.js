import nacl from 'tweetnacl';

export function generateEd25519KeyPair() {
  const kp = nacl.sign.keyPair();
  return {
    publicKey: kp.publicKey, // 32 bytes
    privateKey: kp.secretKey // 64 bytes
  };
}
