import { describe, it, expect } from 'vitest'
import { encryptDM, decryptDM, isEncryptedPayload } from '../src/lib/nip-e2e'
import * as secp from '@noble/secp256k1'

function bytesToHex(b: Uint8Array) {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}

function genPair() {
  const skBytes = crypto.getRandomValues(new Uint8Array(32))
  const pkBytes = secp.schnorr.getPublicKey(skBytes)
  return { sk: bytesToHex(skBytes), pk: bytesToHex(pkBytes) }
}

describe('nip-e2e HKDF+AES-GCM', () => {
  it('roundtrip encrypt/decrypt', async () => {
    const a = genPair()
    const b = genPair()
    const ct = await encryptDM('secret hello', a.sk, b.pk)
    expect(isEncryptedPayload(ct)).toBe(true)
    expect(ct.startsWith('v1.')).toBe(true)
    const pt = await decryptDM(ct, b.sk, a.pk)
    expect(pt).toBe('secret hello')
  })

  it('wrong key fails closed', async () => {
    const a = genPair()
    const b = genPair()
    const c = genPair()
    const ct = await encryptDM('x', a.sk, b.pk)
    const pt = await decryptDM(ct, c.sk, a.pk)
    expect(pt).toBeNull()
  })

  it('isEncryptedPayload detects format', () => {
    expect(isEncryptedPayload('v1.abc.def')).toBe(true)
    expect(isEncryptedPayload('hello')).toBe(false)
  })
})
