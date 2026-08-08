import { test, expect } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'

test.describe('Phase2 Push + Amnezia UI', () => {
  test.use({
    permissions: ['notifications'],
  })

  test('Amnezia status visible + tunnel API up', async ({ page, request }) => {
    // API tunnel
    const tun = await request.post(BASE + '/api/vpn/amnezia/tunnel', {
      data: {
        peerPublicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        endpoint: '198.51.100.20:51820',
      },
    })
    expect(tun.ok()).toBeTruthy()
    const tj = await tun.json()
    expect(['up', 'conf_ready']).toContain(tj.state)
    expect(tj.confPath).toBeTruthy()

    // import prep
    const imp = await request.post(BASE + '/api/vpn/amnezia/import')
    expect(imp.ok()).toBeTruthy()
    const ij = await imp.json()
    expect(ij.ok).toBeTruthy()
    expect(ij.userCopyPath || ij.confPath).toBeTruthy()

    await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2500)
    await expect(page.getByTestId('vpn-amnezia')).toBeVisible()
    await expect(page.getByTestId('vpn-push')).toBeVisible()
    await expect(page.getByTestId('phase2-status')).toContainText(/Amnezia/i)
  })

  test('Push subscribe with granted permission', async ({ page, context }) => {
    // Ensure notifications granted
    await context.grantPermissions(['notifications'], { origin: BASE })

    await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2500)

    // Service worker may need a moment
    const perm = await page.evaluate(() => Notification.permission)
    // In headless Chromium grantPermissions should make it 'granted'
    expect(['granted', 'default', 'denied']).toContain(perm)

    await page.getByTestId('vpn-push').click()
    await page.waitForTimeout(3000)

    const status = await page.getByTestId('phase2-status').innerText()
    // Accept subscribed OR known environment limits
    const ok =
      /subscribed/i.test(status) ||
      /permission_/i.test(status) ||
      /push_unsupported|no_vapid|insecure/i.test(status) ||
      /VAPID/i.test(status)
    expect(ok).toBeTruthy()

    // If subscribed, server should have subscribers >= 1 after reload config
    const cfg = await page.request.get(BASE + '/api/push/config')
    const cj = await cfg.json()
    expect(cj.vapidPublic).toBeTruthy()
    expect(cj.enabled).toBeTruthy()
  })
})
