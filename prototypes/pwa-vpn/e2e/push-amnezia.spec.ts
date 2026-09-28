import { test, expect } from '@playwright/test'
import { apiToken } from './helpers/auth'

const BASE = process.env.APP_URL || 'http://127.0.0.1:5173'
const API = process.env.API_URL || 'http://127.0.0.1:8090'

test.describe('Phase2 Push + Amnezia UI', () => {
  test.use({
    permissions: ['notifications'],
  })

  test('In-app VPN engine visible + tunnel API up', async ({ page, request }) => {
    const token = await apiToken(request)
    const tun = await request.post(API + '/api/vpn/amnezia/tunnel', {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        peerPublicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        endpoint: '198.51.100.20:51820',
      },
    })
    expect(tun.ok()).toBeTruthy()
    const tj = await tun.json()
    expect(['up', 'conf_ready']).toContain(tj.state)
    expect(tj.confPath).toBeTruthy()

    // /api/vpn/amnezia/import removed: used to auto-launch AmneziaVPN.exe

    // Avoid ?dev=1 — known headless loading-overlay gap
    await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(4000)
    await expect(page.getByTestId('vpn-engine')).toBeVisible({ timeout: 15000 })
    await expect(page.getByTestId('vpn-push')).toBeVisible()
  })

  test('Push subscribe with granted permission', async ({ page, context }) => {
    await context.grantPermissions(['notifications'], { origin: BASE })
    await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(4000)
    await expect(page.getByTestId('vpn-push')).toBeVisible({ timeout: 15000 })
    await page.getByTestId('vpn-push').click()
    await page.waitForTimeout(2000)
    await expect(page.getByTestId('vpn-push')).toBeVisible()
  })
})
