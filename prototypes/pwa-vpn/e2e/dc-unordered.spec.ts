import { expect, test } from '@playwright/test'

test('two peer connections exchange one unordered datachannel message', async ({ page }) => {
  await page.goto('about:blank')
  const got = await page.evaluate(async () => {
    const ice = { iceServers: [] as RTCIceServer[] }
    const a = new RTCPeerConnection(ice)
    const b = new RTCPeerConnection(ice)
    const dc = a.createDataChannel('vpn-tunnel', { ordered: false, maxRetransmits: 0 })
    const incoming = new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('dc timeout')), 8000)
      b.ondatachannel = (ev) => {
        ev.channel.onmessage = (m) => {
          clearTimeout(timer)
          resolve(String(m.data))
        }
      }
    })
    b.onicecandidate = (e) => { if (e.candidate) void a.addIceCandidate(e.candidate) }
    a.onicecandidate = (e) => { if (e.candidate) void b.addIceCandidate(e.candidate) }
    const offer = await a.createOffer()
    await a.setLocalDescription(offer)
    await b.setRemoteDescription(offer)
    const answer = await b.createAnswer()
    await b.setLocalDescription(answer)
    await a.setRemoteDescription(answer)
    await new Promise<void>((resolve) => {
      if (dc.readyState === 'open') resolve()
      else dc.onopen = () => resolve()
    })
    dc.send('ping')
    return incoming
  })
  expect(got).toBe('ping')
})
