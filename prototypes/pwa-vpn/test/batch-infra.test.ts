/**
 * Batch tests: VPN-103/107, INF-003/004, MOB-UX-001
 */
import { describe, it, expect } from 'vitest'
import { encodeInvite, decodeInvite, createInvite } from '../src/lib/invite'
import { serializePacket, deserializePacket, StreamType, StreamMultiplexer } from '../src/lib/dc-internals'
import { isMobile, isTouchDevice, vibrate, getSafeAreaInsets } from '../src/lib/mobile-ux'
import { getFallbackTurnServers } from '../src/lib/turn-fallback'

describe('VPN-107 Invite link', () => {
  it('encode + decode roundtrip', () => {
    const data = createInvite('abc123pubkey', ['ws://1.2.3.4:8080'], 'host')
    const link = encodeInvite(data)
    expect(link).toMatch(/^proxi:\/\/invite\//)
    const decoded = decodeInvite(link)
    expect(decoded).not.toBeNull()
    expect(decoded!.pubkey).toBe('abc123pubkey')
    expect(decoded!.role).toBe('host')
    expect(decoded!.endpoints).toEqual(['ws://1.2.3.4:8080'])
  })

  it('decode invalid link returns null', () => {
    expect(decodeInvite('not-a-link')).toBeNull()
    expect(decodeInvite('proxi://invite/!!!invalid')).toBeNull()
  })
})

describe('INF-004 DataChannel packet serialization', () => {
  it('serialize + deserialize roundtrip', () => {
    const pkt = {
      streamId: 42,
      type: StreamType.HTTP,
      flags: 1,
      payload: new Uint8Array([1, 2, 3, 4, 5]),
    }
    const buf = serializePacket(pkt)
    expect(buf.length).toBe(9) // 4 header + 5 payload
    const decoded = deserializePacket(buf)
    expect(decoded.streamId).toBe(42)
    expect(decoded.type).toBe(StreamType.HTTP)
    expect(decoded.flags).toBe(1)
    expect(Array.from(decoded.payload)).toEqual([1, 2, 3, 4, 5])
  })

  it('different stream types', () => {
    for (const type of [StreamType.HTTP, StreamType.SOCKS5, StreamType.DNS, StreamType.RAW, StreamType.HEARTBEAT]) {
      const pkt = { streamId: 1, type, flags: 0, payload: new Uint8Array([0xFF]) }
      const decoded = deserializePacket(serializePacket(pkt))
      expect(decoded.type).toBe(type)
    }
  })

  it('empty payload', () => {
    const pkt = { streamId: 0, type: StreamType.HEARTBEAT, flags: 0, payload: new Uint8Array() }
    const decoded = deserializePacket(serializePacket(pkt))
    expect(decoded.payload.length).toBe(0)
  })
})

describe('VPN-103 TURN fallback', () => {
  it('returns at least one TURN server config', () => {
    const servers = getFallbackTurnServers()
    expect(servers.length).toBeGreaterThan(0)
    expect(servers[0].urls.length).toBeGreaterThan(0)
    expect(servers[0].username.length).toBeGreaterThan(0)
    expect(servers[0].credential.length).toBeGreaterThan(0)
  })
})

describe('MOB-UX-001 mobile helpers', () => {
  it('isMobile returns boolean', () => {
    expect(typeof isMobile()).toBe('boolean')
  })
  it('isTouchDevice returns boolean', () => {
    expect(typeof isTouchDevice()).toBe('boolean')
  })
  it('vibrate does not throw', () => {
    expect(() => vibrate(50)).not.toThrow()
  })
  it('getSafeAreaInsets returns object', () => {
    const insets = getSafeAreaInsets()
    expect(typeof insets.top).toBe('number')
    expect(typeof insets.bottom).toBe('number')
  })
})
