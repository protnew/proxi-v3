/**
 * Same-WiFi dual-role — то, что МОЖНО прогнать без двух сетей.
 * Alice и Bob = два контекста браузера на одном хосте (имитация 2 устройств в одной Wi‑Fi).
 * Dual-network (LTE vs Wi‑Fi) — отдельный ручной сценарий в TEST_PLAYBOOK.md
 */
import { test, expect } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const API = process.env.API_URL || 'http://127.0.0.1:8090'

test.describe('Same-WiFi dual-role (2 browser contexts)', () => {
  test('health + both roles load + in-app engine API', async ({ browser, request }) => {
    const h = await request.get(API + '/api/health')
    expect(h.ok()).toBeTruthy()

    // In-app tunnel without external Amnezia
    const tun = await request.post(API + '/api/vpn/amnezia/tunnel', {
      data: {
        peerPublicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        endpoint: '127.0.0.1:51820',
      },
    })
    expect(tun.ok()).toBeTruthy()
    const tj = await tun.json()
    expect(['up', 'conf_ready']).toContain(tj.state)
    // Product path is userspace when no kernel
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

    await expect(a.getByTestId('vpn-product')).toBeVisible()
    await expect(b.getByTestId('vpn-product')).toBeVisible()
    await expect(a.getByTestId('vpn-give')).toBeVisible()
    await expect(b.getByTestId('vpn-request')).toBeVisible()
    await expect(a.getByTestId('vpn-engine')).toBeVisible()

    // Engine button works in UI
    await a.getByTestId('vpn-engine').click()
    await a.waitForTimeout(2000)
    const st = await a.getByTestId('phase2-status').innerText()
    expect(/up|движок|приложении|userspace|conf/i.test(st)).toBeTruthy()

    await alice.close()
    await bob.close()

    await request.delete(API + '/api/vpn/amnezia/tunnel')
  })
})
