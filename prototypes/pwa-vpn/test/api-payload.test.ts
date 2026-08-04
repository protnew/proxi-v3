import { describe, it, expect } from 'vitest'
import { makeChatPayload } from '../src/lib/api-payload'

describe('makeChatPayload', () => {
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

  it('coerces encrypted flag', () => {
    expect(makeChatPayload('a', 'b', 'x', true).encrypted).toBe(true)
  })
})
