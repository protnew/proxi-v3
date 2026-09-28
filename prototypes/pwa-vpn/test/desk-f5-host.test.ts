import { describe, expect, it } from 'vitest'
import { publishableWTHost } from '../src/lib/vpn-utils.svelte'

describe('F5 wtAddr', () => {
  it('rejects loopback and window.location fallback', () => {
    expect(() => publishableWTHost('127.0.0.1')).toThrow(/wtAddr refused/)
    expect(() => publishableWTHost('')).toThrow(/wtAddr refused/)
    expect(() => publishableWTHost('localhost')).toThrow(/wtAddr refused/)
  })
  it('keeps a reachable host', () => {
    expect(publishableWTHost('203.0.113.9')).toBe('203.0.113.9')
  })
})
