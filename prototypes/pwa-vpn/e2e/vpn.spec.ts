import { test, expect } from '@playwright/test'

/**
 * VPN panel functional smoke — backend /api/vpn/rpc via UI.
 * Expects Go :8080 + Vite :5173 already up (native Windows).
 */
test.describe('VPN panel', () => {
  test('local tunnel connect → connected → disconnect', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Panel visible with modes
    await expect(page.getByText('Локальный тест')).toBeVisible({ timeout: 15000 })
    const connectBtn = page.getByRole('button', { name: /Включить локальный туннель/i })
    await expect(connectBtn).toBeVisible()

    await connectBtn.click()

    // Status becomes connected (bar text)
    await expect(page.getByRole('button', { name: /VPN\s+Подключён/i })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('10.77.0.1')).toBeVisible()
    await expect(page.getByText(/userspace|stub/i).first()).toBeVisible()

    // Disconnect
    await page.getByRole('button', { name: /Отключить VPN/i }).click()
    await expect(page.getByRole('button', { name: /VPN\s+Отключён/i })).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('button', { name: /Включить локальный туннель/i })).toBeVisible()
  })
})
