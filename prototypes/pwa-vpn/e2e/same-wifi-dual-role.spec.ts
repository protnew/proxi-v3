/**
 * Same-WiFi dual-role — то, что МОЖНО прогнать без двух сетей.
 */
import { test, expect } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:5173'
const API = process.env.API_URL || 'http://127.0.0.1:8090'

async function apiToken(request: any): Promise<string> {
  const r = await request.post(API + '/api/auth/signup', {
    data: { npub: 'f'.repeat(64), username: 'e2e_sw_' + Date.now().toString().slice(-6) },
  })
  expect(r.ok()).toBeTruthy()
  const j = await r.json()
  return j.access_token as string
}

test.describe('Same-WiFi dual-role (2 browser contexts)', () => {
  test('health + both roles load + in-app engine API', async ({ browser, request }) => {
    const h = await request.get(API + '/api/health')
    expect(h.ok()).toBeTruthy()

    const token = await apiToken(request)
    const tun = await request.post(API + '/api/vpn/amnezia/tunnel', {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        peerPublicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        endpoint: '127.0.0.1:51820',
      },
    })
    expect(tun.ok()).toBeTruthy()
    const tj = await tun.json()
    expect(['up', 'conf_ready']).toContain(tj.state)
    if (tj.state === 'up') {
      expect(['userspace', 'kernel']).toContain(tj.mode)
    }

    const alice = await browser.newContext()
    const bob = await browser.newContext()
    const a = await alice.newPage()
    const b = await bob.newPage()

    await a.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await b.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
    await a.waitForTimeout(2500)
    await b.waitForTimeout(2500)

    await expect(a.getByTestId('vpn-give')).toBeVisible()
    await expect(b.getByTestId('vpn-give')).toBeVisible()

    await alice.close()
    await bob.close()
  })
})
