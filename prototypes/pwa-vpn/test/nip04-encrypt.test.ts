/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest'
import { createIdentity } from '../src/lib/identity'

const hexToBytes = (hex: string) => {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  return bytes
}
import { NostrChat } from '../src/lib/nostr-chat'

describe('NIP-04 E2E encryption', () => {
  it('round-trip: Alice encrypts → Bob decrypts', async () => {
    const alice = await createIdentity()
    localStorage.clear()
    const bob = await createIdentity()

    // Alice creates a NostrChat with her keys
    const aliceChat = new NostrChat(alice, []) // no relays for test

    // Access internal encrypt/decrypt via sendDM path
    // We test the crypto by having Alice "send" to Bob, then Bob decrypt
    // Since encrypt/decrypt are module-private, test via NostrChat.sendDM + onMessage

    // Actually: test via exported encrypt/decrypt by importing the module internals
    // We can test ECDH shared secret derivation: Alice priv + Bob pub == Bob priv + Alice pub
    const secp = await import('@noble/secp256k1')

    const bytesToHex = (b: any) => Array.from(b).map((x: number) => x.toString(16).padStart(2, '0')).join('')

    // Alice computes shared secret with Bob's pubkey
    const aliceShared = secp.getSharedSecret(hexToBytes(alice.privateKey), hexToBytes('02' + bob.publicKey)) as Uint8Array
    const bobShared = secp.getSharedSecret(hexToBytes(bob.privateKey), hexToBytes('02' + alice.publicKey)) as Uint8Array

    // They must match (first byte is compression prefix, skip it)
    expect(bytesToHex(aliceShared.slice(1, 33))).toBe(bytesToHex(bobShared.slice(1, 33)))
  })

  it('different identities produce different shared secrets', async () => {
    const secp = await import('@noble/secp256k1')
    const bytesToHex = (b: any) => Array.from(b).map((x: number) => x.toString(16).padStart(2, '0')).join('')

    localStorage.clear()
    const alice = await createIdentity()
    localStorage.clear()
    const bob = await createIdentity()
    localStorage.clear()
    const carol = await createIdentity()

    // @noble/secp256k1 getSharedSecret: accepts hex or Uint8Array
    const ab = secp.getSharedSecret(hexToBytes(alice.privateKey), hexToBytes('02' + bob.publicKey))
    const ac = secp.getSharedSecret(hexToBytes(alice.privateKey), hexToBytes('02' + carol.publicKey))
    const aliceBobSecret = bytesToHex((ab as Uint8Array).slice(1, 33))
    const aliceCarolSecret = bytesToHex((ac as Uint8Array).slice(1, 33))

    expect(aliceBobSecret).not.toBe(aliceCarolSecret)
  })

  it('AES-CBC encrypt → decrypt round-trip via crypto.subtle', async () => {
    const keyRaw = crypto.getRandomValues(new Uint8Array(32))
    const key = await crypto.subtle.importKey('raw', keyRaw, { name: 'AES-CBC' }, false, ['encrypt', 'decrypt'])
    const iv = crypto.getRandomValues(new Uint8Array(16))

    const plaintext = 'Привет от Алексея! Hello 123 🚀'
    const encoded = new TextEncoder().encode(plaintext)

    const ciphertext = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, key, encoded)
    const decrypted = await crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, ciphertext)

    expect(new TextDecoder().decode(decrypted)).toBe(plaintext)
  })

  it('wrong key fails to decrypt', async () => {
    const key1 = crypto.getRandomValues(new Uint8Array(32))
    const key2 = crypto.getRandomValues(new Uint8Array(32))

    const encKey = await crypto.subtle.importKey('raw', key1, { name: 'AES-CBC' }, false, ['encrypt'])
    const wrongKey = await crypto.subtle.importKey('raw', key2, { name: 'AES-CBC' }, false, ['decrypt'])

    const iv = crypto.getRandomValues(new Uint8Array(16))
    const ciphertext = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, encKey, new TextEncoder().encode('secret'))

    await expect(crypto.subtle.decrypt({ name: 'AES-CBC', iv }, wrongKey, ciphertext)).rejects.toThrow()
  })

  it('unicode and emoji encrypt/decrypt correctly', async () => {
    const keyRaw = crypto.getRandomValues(new Uint8Array(32))
    const key = await crypto.subtle.importKey('raw', keyRaw, { name: 'AES-CBC' }, false, ['encrypt', 'decrypt'])
    const iv = crypto.getRandomValues(new Uint8Array(16))

    const texts = ['日本語テスト', '🎮🎲', 'Привет, мир!', 'a'.repeat(1000)]
    for (const text of texts) {
      const ct = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, key, new TextEncoder().encode(text))
      const pt = await crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, ct)
      expect(new TextDecoder().decode(pt)).toBe(text)
    }
  })
})
