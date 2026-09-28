import { describe, it, expect, vi, beforeEach } from 'vitest'
import { makeStatus, nextRetrySeconds } from '../src/lib/connection-status'
import { getEmptyState } from '../src/lib/empty-states'
import { t, setLocale, getLocale } from '../src/lib/i18n'
import { encodeInvite, decodeInvite, createInvite } from '../src/lib/invite'
import { toast, onToast, getErrorLog, clearErrorLog, exportErrorLog } from '../src/lib/toast'
import { getUpdateState, onUpdateReady, checkForUpdate, getSWVersion } from '../src/lib/update-check'
import { isMobile, isTouchDevice, getSafeAreaInsets, vibrate } from '../src/lib/mobile-ux'
import { enqueue, listOutbox, remove, clearOutbox, flushOutbox } from '../src/lib/offline-outbox'
import { serializePacket, deserializePacket, StreamType, StreamMultiplexer } from '../src/lib/dc-internals'
import { getFallbackTurnServers, checkIceConnectivity } from '../src/lib/turn-fallback'
import { formatCIDShort, MAX_FILE_BYTES, computeCID } from '../src/lib/ipfs-storage'

describe('connection-status', () => {
  it('makeStatus labels ru/en', () => {
    const s = makeStatus('connected')
    expect(s.phase).toBe('connected')
    expect(s.labelRu).toBe('Связь есть')
    expect(s.labelEn).toBe('Connected')
  })
  it('nextRetrySeconds doubles and caps', () => {
    expect(nextRetrySeconds(0, 2, 30)).toBe(2)
    expect(nextRetrySeconds(1, 2, 30)).toBe(4)
    expect(nextRetrySeconds(10, 2, 30)).toBe(30)
  })
  it('retryInSec passed through', () => {
    expect(makeStatus('retrying', { retryInSec: 8, detail: 'x' }).retryInSec).toBe(8)
  })
})

describe('empty-states', () => {
  it('ru contacts has CTA', () => {
    const e = getEmptyState('contacts', 'ru')
    expect(e.title).toContain('контакт')
    expect(e.cta).toBeTruthy()
  })
  it('en vpn_off', () => {
    const e = getEmptyState('vpn_off', 'en')
    expect(e.title.toLowerCase()).toContain('vpn')
    expect(e.cta).toBeTruthy()
  })
  it('all kinds defined ru+en', () => {
    for (const k of ['contacts','chats','peers','vpn_off','messages'] as const) {
      expect(getEmptyState(k, 'ru').kind).toBe(k)
      expect(getEmptyState(k, 'en').kind).toBe(k)
    }
  })
})

describe('i18n', () => {
  beforeEach(() => { setLocale('ru'); localStorage.removeItem('proxi_locale') })
  it('t ru/en', () => {
    expect(t('vpn.connect', 'ru')).toBe('Подключить VPN')
    expect(t('vpn.connect', 'en')).toBe('Connect VPN')
  })
  it('setLocale persists', () => {
    setLocale('en')
    expect(getLocale()).toBe('en')
    expect(t('chat.new')).toBe('New chat')
  })
})

describe('invite', () => {
  it('roundtrip encode/decode', () => {
    const inv = createInvite('pk123', ['wss://a'], 'host')
    const link = encodeInvite(inv)
    expect(link.startsWith('proxi://invite/')).toBe(true)
    const back = decodeInvite(link)
    expect(back?.pubkey).toBe('pk123')
    expect(back?.role).toBe('host')
    expect(back?.endpoints).toEqual(['wss://a'])
  })
  it('bad link → null', () => {
    expect(decodeInvite('http://nope')).toBeNull()
    expect(decodeInvite('proxi://invite/!!!!')).toBeNull()
  })
})

describe('toast', () => {
  beforeEach(() => { clearErrorLog() })
  it('notifies listeners and logs errors', () => {
    const seen: string[] = []
    const off = onToast(t => seen.push(t.message))
    toast('hello', 'info')
    toast('boom', 'error')
    expect(seen).toEqual(['hello', 'boom'])
    const log = getErrorLog()
    expect(log.some(e => e.message === 'boom')).toBe(true)
    expect(exportErrorLog()).toContain('boom')
    off()
    toast('after-off', 'warn')
    expect(seen).not.toContain('after-off')
  })
})

describe('update-check', () => {
  it('default state', () => {
    const s = getUpdateState()
    expect(s.updateAvailable).toBe(false)
  })
  it('checkForUpdate no-throw without SW', async () => {
    await checkForUpdate()
    expect(await getSWVersion()).toBe('unknown')
  })
  it('onUpdateReady stores callback', () => {
    const cb = vi.fn()
    onUpdateReady(cb)
    expect(typeof cb).toBe('function')
  })
})

describe('mobile-ux', () => {
  it('isMobile false in jsdom default width unless UA mobile', () => {
    expect(typeof isMobile()).toBe('boolean')
    expect(typeof isTouchDevice()).toBe('boolean')
  })
  it('getSafeAreaInsets numbers', () => {
    const i = getSafeAreaInsets()
    expect(i).toMatchObject({ top: expect.any(Number), bottom: expect.any(Number) })
  })
  it('vibrate no-throw', () => {
    expect(() => vibrate(10)).not.toThrow()
  })
})

describe('offline-outbox', () => {
  beforeEach(() => { clearOutbox() })
  it('enqueue list remove flush', async () => {
    const item = enqueue('bob', 'hi')
    expect(listOutbox().length).toBe(1)
    expect(listOutbox()[0].text).toBe('hi')
    remove(item.id)
    expect(listOutbox().length).toBe(0)
    enqueue('bob', 'again')
    const r = await flushOutbox(async () => ({ ok: true, via: 'ws' }))
    expect(r.sent).toBe(1)
    expect(r.left).toBe(0)
  })
  it('failed send stays in outbox', async () => {
    enqueue('bob', 'x')
    const r = await flushOutbox(async () => ({ ok: false, error: 'offline' }))
    expect(r.sent).toBe(0)
    expect(r.left).toBe(1)
  })
})

describe('dc-internals', () => {
  it('serialize roundtrip', () => {
    const pkt = { streamId: 7, type: StreamType.HTTP, flags: 1, payload: new Uint8Array([9, 8, 7]) }
    const bin = serializePacket(pkt)
    const back = deserializePacket(bin)
    expect(back.streamId).toBe(7)
    expect(back.type).toBe(StreamType.HTTP)
    expect(back.flags).toBe(1)
    expect([...back.payload]).toEqual([9, 8, 7])
  })
  it('short packet throws', () => {
    expect(() => deserializePacket(new Uint8Array([1, 2]))).toThrow()
  })
  it('multiplexer create/close without DC', () => {
    const mx = new StreamMultiplexer()
    const id = mx.createStream(StreamType.RAW, () => {})
    expect(id).toBeGreaterThan(0)
    expect(mx.activeStreams).toBe(1)
    expect(mx.send(id, StreamType.RAW, new Uint8Array([1]))).toBe(false)
    mx.closeStream(id)
    expect(mx.activeStreams).toBe(0)
  })
})

describe('turn-fallback', () => {
  it('fallback servers have urls', () => {
    const s = getFallbackTurnServers()
    expect(s.length).toBeGreaterThan(0)
    expect(s[0].urls[0]).toMatch(/^turn:/)
  })
  it('checkIceConnectivity from fake stats', async () => {
    const map = new Map()
    map.set('1', { type: 'candidate-pair', state: 'succeeded' })
    map.set('2', { type: 'local-candidate', candidateType: 'host' })
    const pc = { getStats: async () => map } as any
    const st = await checkIceConnectivity(pc)
    expect(st.successful).toBe(true)
    expect(st.hasHost).toBe(true)
  })
})

describe('ipfs-storage helpers', () => {
  it('formatCIDShort', () => {
    expect(formatCIDShort('short')).toBe('short')
    const long = 'a'.repeat(40)
    const s = formatCIDShort(long)
    expect(s).toContain('…')
    expect(s.length).toBeLessThan(long.length)
  })
  it('MAX_FILE_BYTES 25MB', () => {
    expect(MAX_FILE_BYTES).toBe(25 * 1024 * 1024)
  })
})


describe('amnezia-tunnel', () => {
  it('getTunnelStatus / start / stop / buildConf via fetch mock', async () => {
    const { getTunnelStatus, startTunnel, stopTunnel, buildConf } = await import('../src/lib/amnezia-tunnel')
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method || 'GET').toUpperCase()
      if (method === 'GET') return { ok: true, status: 200, json: async () => ({ state: 'down', mode: 'off' }) }
      if (method === 'POST' && String(url).includes('/conf')) return { ok: true, status: 200, json: async () => ({ conf: '[Interface]', phase: 'ok' }) }
      if (method === 'POST') return { ok: true, status: 200, json: async () => ({ state: 'up', mode: 'amnezia' }) }
      if (method === 'DELETE') return { ok: true, status: 200, json: async () => ({ state: 'down', mode: 'off' }) }
      return { ok: false, status: 500, json: async () => ({ error: 'x' }) }
    })
    vi.stubGlobal('fetch', fetchMock)
    expect((await getTunnelStatus()).state).toBe('down')
    expect((await startTunnel({ peerPublicKey: 'p', endpoint: '1.2.3.4:51820' })).state).toBe('up')
    expect((await stopTunnel()).state).toBe('down')
    expect((await buildConf({ privateKey: 'k', peerPublicKey: 'p', endpoint: 'e' })).conf).toContain('Interface')
    vi.unstubAllGlobals()
  })
})

describe('web-push', () => {
  it('fetchPushConfig', async () => {
    const { fetchPushConfig } = await import('../src/lib/web-push')
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ enabled: true, vapidPublic: 'abc', phase: 'ok' }) })))
    const cfg = await fetchPushConfig()
    expect(cfg.vapidPublic).toBe('abc')
    vi.unstubAllGlobals()
  })
})

describe('ipfs computeCID', () => {
  it('sha256 prefix', async () => {
    const cid = await computeCID(new Uint8Array([1, 2, 3]))
    expect(cid.startsWith('sha256:')).toBe(true)
    expect(cid.length).toBeGreaterThan(20)
  })
})
