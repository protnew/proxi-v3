import { test, expect } from '@playwright/test'

test.describe('Real VPN SOCKS5', () => {
  test('start real tunnel → SOCKS addr → check IP → disconnect', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('domcontentloaded')

    // Expand VPN panel if collapsed
    const bar = page.locator('.vpn-bar, button', { hasText: /VPN/i }).first()
    await expect(bar).toBeVisible({ timeout: 20000 })
    await bar.click()

    // Prefer main action button (works even if radio label differs)
    const start = page.getByRole('button', { name: /настоящий SOCKS5|локальный туннель|Включить/i }).first()
    // If only "Отключить" visible, disconnect first
    const off = page.getByRole('button', { name: /Отключить VPN/i })
    if (await off.isVisible().catch(() => false)) {
      await off.click()
      await page.waitForTimeout(500)
    }

    // Select real mode radio if present
    const realRadio = page.getByRole('radio', { name: /Настоящий|SOCKS5|real/i }).first()
    if (await realRadio.isVisible().catch(() => false)) {
      await realRadio.check()
    }

    const connectBtn = page.getByRole('button', { name: /Включить настоящий SOCKS5|Включить локальный|Подключить/i }).first()
    await expect(connectBtn).toBeVisible({ timeout: 10000 })
    await connectBtn.click()

    await expect(page.locator('.vpn-bar, button').filter({ hasText: /ON|Подключён|SOCKS/i }).first()).toBeVisible({ timeout: 15000 })
    await expect(page.getByText(/127\.0\.0\.1:10808|10\.77\.0\.1/)).toBeVisible({ timeout: 10000 })

    const check = page.getByRole('button', { name: /Проверить IP/i })
    if (await check.isVisible().catch(() => false)) {
      await check.click()
      await expect(page.getByText(/IP через туннель|\d+\.\d+\.\d+\.\d+/i).first()).toBeVisible({ timeout: 25000 })
    }

    await page.getByRole('button', { name: /Отключить VPN/i }).click()
    await expect(page.locator('.vpn-bar, button').filter({ hasText: /Отключён/i }).first()).toBeVisible({ timeout: 10000 })
  })
})
