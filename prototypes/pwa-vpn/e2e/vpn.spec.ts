import { test, expect } from '@playwright/test'

test.describe('Real VPN SOCKS5', () => {
  test('start real tunnel → SOCKS addr → check IP → disconnect', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('domcontentloaded')

    const bar = page.locator('button.vpn-bar')
    await expect(bar).toBeVisible({ timeout: 20000 })

    // Ensure panel expanded (details visible)
    async function ensureExpanded() {
      const details = page.locator('.vpn-details')
      if (!(await details.isVisible().catch(() => false))) {
        await bar.click()
      }
      await expect(details).toBeVisible({ timeout: 5000 })
    }
    await ensureExpanded()

    // Reset if already on
    const offBtn = page.locator('button.toggle-btn', { hasText: /Отключить/i })
    if (await offBtn.isVisible().catch(() => false)) {
      await offBtn.click()
      await expect(bar).toContainText(/Отключён/i, { timeout: 10000 })
      await ensureExpanded()
    }

    // Select real mode
    const real = page.locator('label', { hasText: /Настоящий/i })
    if (await real.isVisible().catch(() => false)) await real.click()

    const onBtn = page.locator('button.toggle-btn')
    await expect(onBtn).toBeVisible()
    await onBtn.click()

    await expect(bar).toContainText(/ON|Подключён|SOCKS/i, { timeout: 15000 })
    await ensureExpanded()
    await expect(page.locator('.vpn-details')).toContainText('127.0.0.1:10808')

    const check = page.locator('button.check-btn')
    if (await check.isVisible().catch(() => false)) {
      await check.click()
      await expect(page.locator('.ip-box')).toBeVisible({ timeout: 25000 })
    }

    await page.locator('button.toggle-btn', { hasText: /Отключить/i }).click()
    await expect(bar).toContainText(/Отключён/i, { timeout: 10000 })
  })
})
