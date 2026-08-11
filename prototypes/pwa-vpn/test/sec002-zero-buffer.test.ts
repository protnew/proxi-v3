import { describe, it, expect } from 'vitest'
import { zeroBuffer } from '../src/lib/nip-e2e'

describe('SEC-002: Crypto-erasure', () => {
  it('zeroBuffer fills Uint8Array with zeros', () => {
    const buf = new Uint8Array([1, 2, 3, 4, 5, 255, 128, 64])
    expect(buf.some(b => b !== 0)).toBe(true)
    zeroBuffer(buf)
    expect(buf.every(b => b === 0)).toBe(true)
  })

  it('zeroBuffer handles empty array', () => {
    const buf = new Uint8Array(0)
    zeroBuffer(buf)
    expect(buf.length).toBe(0)
  })

  it('zeroBuffer handles null/undefined gracefully', () => {
    expect(() => zeroBuffer(null)).not.toThrow()
    expect(() => zeroBuffer(undefined)).not.toThrow()
  })

  it('zeroBuffer wipes a 32-byte key', () => {
    const key = new Uint8Array(32)
    for (let i = 0; i < 32; i++) key[i] = i + 1
    expect(key.some(b => b !== 0)).toBe(true)
    zeroBuffer(key)
    expect(key.every(b => b === 0)).toBe(true)
    expect(key.length).toBe(32) // length preserved
  })
})
