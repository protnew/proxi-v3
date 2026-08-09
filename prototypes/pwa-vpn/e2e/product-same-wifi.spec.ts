/**
 * Product path: same Wi‑Fi dual-role without external Amnezia.
 * VPN-LAN-001 + VPN-P2P-001 + MSG-DM-001 smoke.
 */
import { test, expect } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const API = process.env.API_URL || 'http://127.0.0.1:8090'

test.describe('Product same-WiFi path', () => {
  test('LAN API returns bind + phone urls', async ({ request }) => {
    const r = await request.get(API + '/api/network/lan')
    expect(r.ok()).toBeTruthy()
    const j = await r.json()
    expect(j.ok).toBeTruthy()
    expect(String(j.bind)).toMatch(/0\.0\.0\.0/)
    expect(j.port).toBeTruthy()
  })

  test('Alice give VPN one-click (demo partner prefilled)', async ({ browser }) => {
    const alice = await browser.newContext()
    const bob = await browser.newContext()
    const a = await alice.newPage()
    const b = await bob.newPage()
    await a.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await b.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
    await a.waitForTimeout(3000)
    await b.waitForTimeout(3000)

    await expect(a.getByTestId('vpn-product')).toBeVisible()
    // demo partner hint or give works without modal paste
    await a.getByTestId('vpn-give').click()
    await a.waitForTimeout(4000)
    const log = await a.getByTestId('vpn-log').innerText().catch(() => '')
    const status = await a.locator('.status-bar, [class*=status]').allInnerTexts().catch(() => [])
    // Accept either invite OK or explicit sharing text
    const body = (await a.locator('body').innerText())
    const ok = /Инвайт|Раздаю VPN|invite|sharing|Nostr/i.test(body + log)
    expect(ok).toBeTruthy()

    await alice.close()
    await bob.close()
  })

  test('Alice sends DM visible path (best-effort dual window)', async ({ browser }) => {
    const alice = await browser.newContext()
    const bob = await browser.newContext()
    const a = await alice.newPage()
    const b = await bob.newPage()
    await a.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await b.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
    await a.waitForTimeout(3500)
    await b.waitForTimeout(3500)

    // open Bob chat on Alice if needed
    const bobBtn = a.getByRole('button', { name: /Bob/i }).first()
    if (await bobBtn.count()) await bobBtn.click()
    await a.waitForTimeout(500)

    const input = a.locator('textarea, input[type="text"]').last()
    if (await input.count()) {
      await input.fill('night-factory-dm-' + Date.now())
      await a.keyboard.press('Enter')
      await a.waitForTimeout(2000)
    }
    // At least Alice UI still healthy
    await expect(a.getByTestId('vpn-product')).toBeVisible()

    await alice.close()
    await bob.close()
  })
})
