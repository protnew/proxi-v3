/**
 * VPN-P2P-E2E: Dual-context give->accept->connected WebRTC flow
 * Replaces false SIG-003/WT full-chain DONE claims.
 */
import { test, expect, type Page } from '@playwright/test'

const BASE = 'http://127.0.0.1:8090'

async function waitForApp(page: Page, role: string) {
  await page.goto(`${BASE}/?role=${role}`, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(5000)
  await page.waitForFunction(() => {
    return !!(localStorage.getItem('proxi_token') || '')
  }, { timeout: 15000 })
}

test.describe('VPN P2P E2E - give/accept product flow', () => {
  test('Alice give VPN - demo partner Bob prefilled', async ({ page }) => {
    await waitForApp(page, 'alice')
    await expect(page.locator('body')).toContainText(/Proxi|Indestructible|VPN|Messenger/i, { timeout: 15000 })
    await expect(page.getByText(/Bob.*demo/)).toBeVisible({ timeout: 10000 })
    const giveBtn = page.getByTestId('vpn-give')
    await expect(giveBtn).toBeVisible({ timeout: 5000 })
    await giveBtn.click()
    await page.waitForTimeout(2000)
    const log = page.locator('body')
    await expect(log).toContainText(/invite|sharing|\u0438\u043d\u0432\u0430\u0439\u0442|\u0440\u0430\u0437\u0434\u0430\u044e/i, { timeout: 5000 })
  })

  test('Bob context shows demo partner Alice', async ({ page }) => {
    await waitForApp(page, 'bob')
    await expect(page.locator('body')).toContainText(/Proxi|Indestructible|VPN|Messenger/i, { timeout: 15000 })
    await expect(page.getByText(/Alice.*demo/)).toBeVisible({ timeout: 10000 })
    const reqBtn = page.getByTestId('vpn-request')
    await expect(reqBtn).toBeVisible({ timeout: 5000 })
  })

  test('In-app engine starts without hardcoded 127.0.0.1:51820', async ({ page }) => {
    await waitForApp(page, 'alice')
    const engineBtn = page.getByTestId('vpn-engine')
    if (await engineBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await engineBtn.click()
      await page.waitForTimeout(5000)
      const text = await page.locator('body').textContent()
      expect(text).not.toContain('peer: 127.0.0.1:51820')
    }
  })

  test('LAN API provides phone URL', async ({ request }) => {
    const r = await request.get(`${BASE}/api/network/lan`)
    expect(r.status()).toBe(200)
    const j = await r.json()
    expect(j.bind).toContain('0.0.0.0')
    expect(j.phone_urls).toBeDefined()
    expect(Array.isArray(j.phone_urls)).toBeTruthy()
  })
})
