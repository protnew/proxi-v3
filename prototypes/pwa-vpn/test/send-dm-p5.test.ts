/**
 * @vitest-environment jsdom
 * P5: primary WS DM path must put NIP-44 ciphertext on the wire when E2E is on.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { webcrypto } from 'crypto'
import * as secp from '@noble/secp256k1'

vi.stubGlobal('crypto', webcrypto)

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

const ls: Record<string, string> = {}
vi.stubGlobal('localStorage', {
  getItem: (k: string) => ls[k] ?? null,
  setItem: (k: string, v: string) => { ls[k] = v },
  removeItem: (k: string) => { delete ls[k] },
  clear: () => Object.keys(ls).forEach(k => delete ls[k]),
})

class MockWS {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  readyState = MockWS.CONNECTING
  onopen: ((ev?: any) => void) | null = null
  onmessage: ((ev: any) => void) | null = null
  onerror: ((ev?: any) => void) | null = null
  onclose: ((ev?: any) => void) | null = null
  sent: any[] = []
  url: string
  constructor(url: string) {
    this.url = url
    MockWS.instances.push(this)
    queueMicrotask(() => {
      this.readyState = MockWS.OPEN
      this.onopen?.({})
    })
  }
  send(data: any) { this.sent.push(data) }
  close() { this.readyState = MockWS.CLOSED; this.onclose?.({}) }
  static instances: MockWS[] = []
  static reset() { MockWS.instances = [] }
}
vi.stubGlobal('WebSocket', MockWS as any)

function ok(data: any = {}) {
  return {
    ok: true, status: 200,
    json: async () => data,
    text: async () => JSON.stringify(data),
    headers: new Headers(),
  } as any
}

function bytesToHex(b: Uint8Array) {
  return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join('')
}
function genPair() {
  const skBytes = crypto.getRandomValues(new Uint8Array(32))
  const pkBytes = secp.schnorr.getPublicKey(skBytes)
  return { sk: bytesToHex(skBytes), pk: bytesToHex(pkBytes) }
}

import {
  setIdentity, connectRelays, sendDM, setE2EEnabledLocal,
} from '../src/lib/api'
import { makeChatPayload as makePayload } from '../src/lib/api-payload'
import { isNip44Payload, NIP44_PREFIX, decryptDM } from '../src/lib/nip-e2e'

beforeEach(() => {
  vi.clearAllMocks()
  MockWS.reset()
  Object.keys(ls).forEach(k => delete ls[k])
  mockFetch.mockResolvedValue(ok({ access_token: 'jwt', user_id: 'u1' }))
  ls['proxi_token'] = 'jwt'
})

describe('P5 sendDM primary WS E2E', () => {
  it('makeChatPayload never sets encrypted:true on plaintext', () => {
    const p = makePayload('from', 'to', 'hello plaintext', true)
    expect(p.encrypted).toBe(false)
    expect(p.text).toBe('hello plaintext')
  })

  it('makeChatPayload sets encrypted:true only for nip44 ciphertext', () => {
    const ct = NIP44_PREFIX + 'abc'
    const p = makePayload('from', 'to', ct, true)
    expect(p.encrypted).toBe(true)
  })

  it('sendDM with E2E on puts nip44 ciphertext on WS (no plaintext)', async () => {
    const alice = genPair()
    const bob = genPair()
    setIdentity(alice.pk, alice.sk)
    setE2EEnabledLocal(true)

    await connectRelays()
    await new Promise(r => setTimeout(r, 25))

    const secret = 'P5-secret-alice-to-bob-неубиваемый'
    const r = await sendDM(bob.pk, secret)
    expect(r.status).toBe(200)
    expect((r as any).data?.via).toBe('ws')
    expect((r as any).data?.encrypted).toBe(true)

    const ws = MockWS.instances.at(-1)!
    const chatFrames = ws.sent
      .map(s => { try { return JSON.parse(String(s)) } catch { return null } })
      .filter(m => m && m.type === 'chat')
    expect(chatFrames.length).toBeGreaterThanOrEqual(1)
    const frame = chatFrames[chatFrames.length - 1]
    expect(frame.text).not.toBe(secret)
    expect(frame.text.includes(secret)).toBe(false)
    expect(isNip44Payload(frame.text)).toBe(true)
    expect(frame.encrypted).toBe(true)

    const pt = await decryptDM(frame.text, bob.sk, alice.pk)
    expect(pt).toBe(secret)
  })

  it('sendDM with E2E on refuses when peer pubkey too short', async () => {
    const alice = genPair()
    setIdentity(alice.pk, alice.sk)
    setE2EEnabledLocal(true)
    await connectRelays()
    await new Promise(r => setTimeout(r, 20))
    const r = await sendDM('short-peer', 'nope')
    expect(r.status).toBe(400)
    expect((r as any).error).toMatch(/e2e_required/)
    const ws = MockWS.instances.at(-1)!
    const chatFrames = ws.sent
      .map(s => { try { return JSON.parse(String(s)) } catch { return null } })
      .filter(m => m && m.type === 'chat')
    expect(chatFrames.length).toBe(0)
  })

  it('sendDM with E2E off may send plaintext and encrypted:false', async () => {
    const alice = genPair()
    setIdentity(alice.pk, alice.sk)
    setE2EEnabledLocal(false)
    await connectRelays()
    await new Promise(r => setTimeout(r, 20))
    const r = await sendDM(genPair().pk, 'plain-ok')
    expect(r.status).toBe(200)
    const ws = MockWS.instances.at(-1)!
    const chatFrames = ws.sent
      .map(s => { try { return JSON.parse(String(s)) } catch { return null } })
      .filter(m => m && m.type === 'chat')
    expect(chatFrames.at(-1).text).toBe('plain-ok')
    expect(chatFrames.at(-1).encrypted).toBe(false)
  })
})
