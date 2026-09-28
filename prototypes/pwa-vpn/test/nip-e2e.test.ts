import { describe, it, expect } from 'vitest'
import { encryptDM, decryptDM, isEncryptedPayload, isNip44Payload, NIP44_PREFIX } from '../src/lib/nip-e2e'
import * as secp from '@noble/secp256k1'

function bytesToHex(b: Uint8Array) {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}
function genPair() {
  const skBytes = crypto.getRandomValues(new Uint8Array(32))
  const pkBytes = secp.schnorr.getPublicKey(skBytes)
  return { sk: bytesToHex(skBytes), pk: bytesToHex(pkBytes) }
}

describe('NIP-44 v2 (SL-033)', () => {
  it('roundtrip encrypt/decrypt', async () => {
    const a = genPair()
    const b = genPair()
    const ct = await encryptDM('secret hello nip44', a.sk, b.pk)
    expect(isNip44Payload(ct)).toBe(true)
    expect(ct.startsWith(NIP44_PREFIX)).toBe(true)
    expect(isEncryptedPayload(ct)).toBe(true)
    const pt = await decryptDM(ct, b.sk, a.pk)
    expect(pt).toBe('secret hello nip44')
  })

  it('wrong key fails closed', async () => {
    const a = genPair()
    const b = genPair()
    const c = genPair()
    const ct = await encryptDM('x', a.sk, b.pk)
    const pt = await decryptDM(ct, c.sk, a.pk)
    expect(pt).toBeNull()
  })

  it('unicode payload', async () => {
    const a = genPair()
    const b = genPair()
    const msg = 'Привет 🌍 VPN'
    const ct = await encryptDM(msg, a.sk, b.pk)
    expect(await decryptDM(ct, b.sk, a.pk)).toBe(msg)
  })
})
