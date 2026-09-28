/** Import identity from nsec/hex. Split from identity.ts (TZ-EXEC-20260924 A1). */
import * as secp from '@noble/secp256k1'
import { encodeBech32, decodeBech32 } from './identity_nostr'
import { bytesToHex, hexToBytes, persistIdentitySecure, type Identity } from './identity'

export async function importIdentity(input: string): Promise<Identity> {
  const raw = input.trim()
  let privateKeyHex: string
  if (raw.startsWith('nsec1') || raw.startsWith('NSEC1')) {
    const { hrp, data } = decodeBech32(raw)
    if (hrp !== 'nsec') throw new Error('expected nsec')
    if (data.length < 32) throw new Error('nsec payload too short')
    privateKeyHex = bytesToHex(data.slice(0, 32))
  } else if (/^[0-9a-fA-F]{64}$/.test(raw)) {
    privateKeyHex = raw.toLowerCase()
  } else {
    throw new Error('unsupported key format (use nsec1… or 64-hex)')
  }
  const privateKeyBytes = hexToBytes(privateKeyHex)
  const publicKey = bytesToHex(secp.schnorr.getPublicKey(privateKeyBytes))
  const identity: Identity = {
    privateKey: privateKeyHex,
    publicKey,
    npub: encodeBech32('npub', hexToBytes(publicKey)),
    nsec: encodeBech32('nsec', privateKeyBytes),
    createdAt: Date.now(),
  }
  await persistIdentitySecure(identity)
  return identity
}
