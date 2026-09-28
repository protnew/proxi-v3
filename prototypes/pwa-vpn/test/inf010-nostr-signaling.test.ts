import { describe, it, expect, vi, beforeEach } from 'vitest'
import { NostrSignaling, KIND_VPN_REQUEST, KIND_VPN_OFFER } from '../src/lib/nostr-signaling'
import type { Identity } from '../src/lib/identity'

// Mock WebSocket
class MockWebSocket {
  static instances: MockWebSocket[] = []
  static OPEN = 1
  
  url: string
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onerror: ((e: any) => void) | null = null
  sent: string[] = []
  
  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
    // Simulate async connection
    setTimeout(() => {
      this.readyState = 1
      this.onopen?.()
    }, 10)
  }
  
  send(data: string) {
    this.sent.push(data)
  }
  
  close() {
    this.readyState = 3
  }
  
  // Simulate receiving a message
  simulateReceive(data: string) {
    this.onmessage?.({ data })
  }
}

// Patch global WebSocket
global.WebSocket = MockWebSocket as any

// Mock identity signing
vi.mock('../src/lib/identity', () => ({
  signEvent: vi.fn().mockResolvedValue('a'.repeat(128)),
  createIdentity: vi.fn(),
}))

describe('INF-010: Nostr Signaling', () => {
  beforeEach(() => {
    MockWebSocket.instances = []
  })

  it('connects to configured relays on start', async () => {
    const identity: Identity = {
      publicKey: 'a'.repeat(64),
      privateKey: 'b'.repeat(64),
      npub: 'npub1test',
      nsec: 'nsec1test',
      mnemonic: '',
    }

    const signaling = new NostrSignaling(identity, ['wss://relay.test'])
    await signaling.start(() => {})

    // Wait for async WebSocket connection
    await new Promise(r => setTimeout(r, 50))

    expect(MockWebSocket.instances.length).toBe(1)
    expect(MockWebSocket.instances[0].url).toBe('wss://relay.test')
    signaling.stop()
  })

  it('publishes VPN request with correct event format', async () => {
    const identity: Identity = {
      publicKey: 'a'.repeat(64),
      privateKey: 'b'.repeat(64),
      npub: 'npub1test',
      nsec: 'nsec1test',
      mnemonic: '',
    }

    const signaling = new NostrSignaling(identity, ['wss://relay.test'])
    await signaling.start(() => {})
    await new Promise(r => setTimeout(r, 50))

    // Send request
    await signaling.sendRequest('c'.repeat(64))

    // Check that a message was sent
    const ws = MockWebSocket.instances[0]
    expect(ws.sent.length).toBeGreaterThan(0)

    // The sent message should be ['REQ', subId, filter] and ['EVENT', event]
    // Find the EVENT message
    const eventMsg = ws.sent.find(s => s.startsWith('["EVENT"'))
    expect(eventMsg).toBeDefined()

    const parsed = JSON.parse(eventMsg!)
    expect(parsed[0]).toBe('EVENT')
    expect(parsed[1].kind).toBe(KIND_VPN_REQUEST)
    expect(parsed[1].tags).toEqual([['p', 'c'.repeat(64)]])
    expect(parsed[1].content).toContain('request')

    signaling.stop()
  })

  it('receives VPN signal events from relay', async () => {
    const identity: Identity = {
      publicKey: 'a'.repeat(64),
      privateKey: 'b'.repeat(64),
      npub: 'npub1test',
      nsec: 'nsec1test',
      mnemonic: '',
    }

    let receivedSignal: any = null
    const signaling = new NostrSignaling(identity, ['wss://relay.test'])
    await signaling.start((signal) => {
      receivedSignal = signal
    })
    await new Promise(r => setTimeout(r, 50))

    // Simulate relay sending an EVENT message
    const fakeEvent = {
      id: '1'.repeat(64),
      pubkey: 'c'.repeat(64),
      created_at: Math.floor(Date.now() / 1000),
      kind: KIND_VPN_OFFER,
      tags: [['p', 'a'.repeat(64)]],
      content: JSON.stringify({ type: 'offer', sdp: 'fake-sdp-data' }),
      sig: 'a'.repeat(128),
    }

    const relayMsg = JSON.stringify(['EVENT', 'sub-id', fakeEvent])
    MockWebSocket.instances[0].simulateReceive(relayMsg)

    // Wait for async handling
    await new Promise(r => setTimeout(r, 50))

    expect(receivedSignal).not.toBeNull()
    expect(receivedSignal.type).toBe('offer')
    expect(receivedSignal.sdp).toBe('fake-sdp-data')
    expect(receivedSignal.from).toBe('c'.repeat(64))

    signaling.stop()
  })

  it('sendOffer publishes SDP to target via Nostr relay', async () => {
    const identity: Identity = {
      publicKey: 'a'.repeat(64),
      privateKey: 'b'.repeat(64),
      npub: 'npub1test',
      nsec: 'nsec1test',
      mnemonic: '',
    }

    const signaling = new NostrSignaling(identity, ['wss://relay.test'])
    await signaling.start(() => {})
    await new Promise(r => setTimeout(r, 50))

    await signaling.sendOffer('d'.repeat(64), 'test-sdp-offer-string')

    const ws = MockWebSocket.instances[0]
    const eventMsg = ws.sent.find(s => s.startsWith('["EVENT"'))
    const parsed = JSON.parse(eventMsg!)
    
    expect(parsed[1].kind).toBe(KIND_VPN_OFFER)
    expect(parsed[1].tags).toEqual([['p', 'd'.repeat(64)]])
    const content = JSON.parse(parsed[1].content)
    expect(content.type).toBe('offer')
    expect(content.sdp).toBe('test-sdp-offer-string')

    signaling.stop()
  })
})
