import { afterEach, describe, expect, it } from 'vitest'
import { WebRTCVPNClient } from '../src/lib/webrtc-vpn'

describe('vpn-tunnel datachannel', () => {
  const prev = globalThis.RTCPeerConnection
  afterEach(() => { globalThis.RTCPeerConnection = prev })

  it('opens vpn-tunnel unordered with zero retransmits', async () => {
    const calls: Array<[string, Record<string, unknown>]> = []
    class FakePC {
      iceGatheringState = 'complete'
      createDataChannel(label: string, opts: Record<string, unknown>) {
        calls.push([label, opts])
        return { binaryType: '', onmessage: null, onclose: null, onerror: null }
      }
      createOffer() { return Promise.resolve({ type: 'offer', sdp: 'v=0' }) }
      setLocalDescription() { return Promise.resolve() }
    }
    globalThis.RTCPeerConnection = FakePC as unknown as typeof RTCPeerConnection
    const client = new WebRTCVPNClient()
    await client.createOffer()
    expect(calls[0][0]).toBe('vpn-tunnel')
    expect(calls[0][1]).toEqual({ ordered: false, maxRetransmits: 0 })
  })
})
