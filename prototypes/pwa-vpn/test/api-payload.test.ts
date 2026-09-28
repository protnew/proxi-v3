import { describe, it, expect } from 'vitest'
import { makeChatPayload } from '../src/lib/api-payload'
import { NIP44_PREFIX } from '../src/lib/nip-e2e'

describe('makeChatPayload (P5)', () => {
  it('builds chat payload shape', () => {
    const p = makeChatPayload('alice', 'bob', 'hi', false, 1000)
    expect(p).toEqual({
      type: 'chat',
      from: 'alice',
      to: 'bob',
      text: 'hi',
      ts: 1000,
      encrypted: false,
    })
  })

  it('refuses encrypted:true on plaintext (P5)', () => {
    expect(makeChatPayload('a', 'b', 'x', true).encrypted).toBe(false)
  })

  it('sets encrypted:true only for nip44 ciphertext', () => {
    const ct = NIP44_PREFIX + 'abc'
    expect(makeChatPayload('a', 'b', ct, true).encrypted).toBe(true)
    expect(makeChatPayload('a', 'b', ct, false).encrypted).toBe(false)
  })
})
