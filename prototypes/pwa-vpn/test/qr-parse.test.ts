import { describe, it, expect } from 'vitest'
import { parseQRContent } from '../src/lib/qr'

describe('parseQRContent', () => {
  it('parses JSON nostr contact', () => {
    const raw = JSON.stringify({ type: 'nostr', pubkey: 'npub1abc', name: 'Bob' })
    const d = parseQRContent(raw)
    expect(d).not.toBeNull()
    expect(d!.pubkey).toBe('npub1abc')
  })

  it('parses nostr: URI', () => {
    const d = parseQRContent('nostr:npub1xyz')
    expect(d).toEqual({ type: 'nostr', pubkey: 'npub1xyz' })
  })

  it('returns null for garbage', () => {
    expect(parseQRContent('not-a-qr')).toBeNull()
    expect(parseQRContent('{"type":"x"}')).toBeNull()
  })
})
