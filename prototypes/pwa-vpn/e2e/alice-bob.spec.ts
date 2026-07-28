import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://localhost:5173'
const API = process.env.API_URL || 'http://localhost:8080'

async function waitForApp(page: Page) {
  await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })
  await expect(page.getByText('Indestructible Messenger')).toBeVisible({ timeout: 20000 })
  await page.waitForFunction(() => {
    try {
      return !!(localStorage.getItem('proxi_token') || '') && !!(window as any).__proxiPubkey
    } catch { return false }
  }, { timeout: 25000 })
}

async function pubkeyOf(page: Page): Promise<string> {
  const pk = await page.evaluate(() => (window as any).__proxiPubkey || '')
  expect(pk.length).toBeGreaterThanOrEqual(32)
  return pk
}

async function startChat(page: Page, peer: string, name: string) {
  await page.getByRole('button', { name: '✏️' }).click()
  await expect(page.getByText('Новый чат')).toBeVisible({ timeout: 5000 })
  const userId = page.getByRole('textbox', { name: /User ID/i })
  if (await userId.count()) await userId.fill(peer)
  else await page.locator('input').first().fill(peer)
  const nameBox = page.getByRole('textbox', { name: /Имя/i })
  if (await nameBox.count()) await nameBox.fill(name)
  else {
    const ph = page.locator('input[placeholder*="Имя"]')
    if (await ph.count()) await ph.fill(name)
  }
  await page.getByRole('button', { name: /Начать чат/i }).click()
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 8000 })
}

test.describe('Alice → Bob DM (two contexts, native Go WS)', () => {
  test('message from Alice appears for Bob', async ({ browser }) => {
    const health = await fetch(`${API}/api/health`).then(r => r.status).catch(() => 0)
    expect(health, 'native Go server on :8080 required (no Docker)').toBe(200)

    const aliceCtx = await browser.newContext()
    const bobCtx = await browser.newContext()
    const alice = await aliceCtx.newPage()
    const bob = await bobCtx.newPage()

    try {
      await waitForApp(alice)
      await waitForApp(bob)
      const alicePk = await pubkeyOf(alice)
      const bobPk = await pubkeyOf(bob)
      expect(alicePk).not.toBe(bobPk)

      await startChat(bob, alicePk, 'Alice')
      await startChat(alice, bobPk, 'Bob')

      const text = `Alice→Bob UI ${Date.now()}`
      const box = alice.getByRole('textbox', { name: /Сообщение/i }).or(alice.locator('textarea').first())
      await box.fill(text)
      await box.press('Enter')

      await expect(alice.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 8000 })
      await expect(bob.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 15000 })
    } finally {
      await aliceCtx.close()
      await bobCtx.close()
    }
  })
})
