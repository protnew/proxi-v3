import { test, expect } from '@playwright/test'

test.describe('Real VPN SOCKS5', () => {
  test('start real tunnel → SOCKS addr → check IP → disconnect', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(/Настоящий SOCKS5/i)).toBeVisible({ timeout: 15000 })
    await page.getByRole('button', { name: /Включить настоящий SOCKS5/i }).click()

    await expect(page.getByRole('button', { name: /VPN\s+ON/i })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('127.0.0.1:10808')).toBeVisible()
    await expect(page.getByText(/REAL|socks5/i).first()).toBeVisible()

    await page.getByRole('button', { name: /Проверить IP через VPN/i }).click()
    await expect(page.getByText(/IP через туннель/i)).toBeVisible({ timeout: 20000 })

    await page.getByRole('button', { name: /Отключить VPN/i }).click()
    await expect(page.getByRole('button', { name: /VPN\s+Отключён/i })).toBeVisible({ timeout: 10000 })
  })
})
