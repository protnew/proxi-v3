/**
 * ONB-000: first visit without keys → AuthScreen → create → enter app
 */
import { test, expect } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:5173'

async function wipeClientState(page: import('@playwright/test').Page) {
  await page.evaluate(async () => {
    try { localStorage.clear() } catch {}
    try { sessionStorage.clear() } catch {}
    try {
      if (indexedDB.databases) {
        const dbs = await indexedDB.databases()
        await Promise.all((dbs || []).map(db => db.name ? new Promise<void>((res, rej) => {
          const req = indexedDB.deleteDatabase(db.name!)
          req.onsuccess = () => res()
          req.onerror = () => rej(req.error)
          req.onblocked = () => res()
        }) : Promise.resolve()))
      }
    } catch {}
  })
}

test.describe('ONB-000 identity genesis', () => {
  test('fresh profile shows AuthScreen and can create account', async ({ browser }) => {
    test.setTimeout(60000)
    const ctx = await browser.newContext()
    const page = await ctx.newPage()
    await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })
    await wipeClientState(page)
    await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })

    await expect(page.getByTestId('auth-screen')).toBeVisible({ timeout: 20000 })
    await page.getByTestId('auth-create').click()
    await expect(page.getByTestId('auth-nsec')).toBeVisible({ timeout: 10000 })
    await page.getByTestId('auth-backup-ok').check()
    await page.getByTestId('auth-enter').click()

    await expect(
      page.getByTestId('chat-empty-brand').or(page.getByRole('heading', { name: 'Proxi' })).first()
    ).toBeVisible({ timeout: 25000 })
    await expect(page.getByText(/Выберите чат/)).toBeVisible()
    await page.screenshot({ path: 'test-results/onb000-after-create.png', fullPage: true })
    await ctx.close()
  })
})
