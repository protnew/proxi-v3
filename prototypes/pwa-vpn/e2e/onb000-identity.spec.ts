/**
 * ONB-000: first visit without keys → AuthScreen → create → enter app
 */
import { test, expect } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:8090'

test.describe('ONB-000 identity genesis', () => {
  test('fresh profile shows AuthScreen and can create account', async ({ browser }) => {
    test.setTimeout(60000)
    const ctx = await browser.newContext()
    const page = await ctx.newPage()
    // Ensure empty storage before navigation
    await page.goto(APP_URL)
    await page.evaluate(() => {
      localStorage.clear()
      sessionStorage.clear()
    })
    await page.reload({ waitUntil: 'domcontentloaded' })

    await expect(page.getByTestId('auth-screen')).toBeVisible({ timeout: 20000 })
    await page.getByTestId('auth-create').click()
    await expect(page.getByTestId('auth-nsec')).toBeVisible({ timeout: 10000 })
    await page.getByTestId('auth-backup-ok').check()
    await page.getByTestId('auth-enter').click()

    // After reload should reach messenger
    await expect(page.getByRole('heading', { name: 'Proxi', level: 2 })).toBeVisible({ timeout: 25000 })
    await expect(page.getByText('Выберите чат')).toBeVisible()
    await page.screenshot({ path: 'test-results/onb000-after-create.png', fullPage: true })
    await ctx.close()
  })
})
