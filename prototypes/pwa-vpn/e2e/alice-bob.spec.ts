import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:5173'
const API = process.env.API_URL || 'http://127.0.0.1:8090'

async function waitForWs(page: Page) {
  await page.waitForFunction(() => {
    try { return !!(window as any).__proxiGetStatus?.()?.connected } catch { return false }
  }, null, { timeout: 30000 })
}

async function boot(page: Page) {
  // ?dev=1: __proxiPubkey debug globals are dev-gated (P25)
  await page.goto(APP_URL + '/?dev=1', { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(2000)
  await page.waitForFunction(() => !!(window as any).__proxiPubkey, null, { timeout: 25000 })
  await waitForWs(page)
}

async function pubkeyOf(page: Page): Promise<string> {
  const pk = await page.evaluate(() => (window as any).__proxiPubkey || '')
  expect(pk.length).toBeGreaterThanOrEqual(32)
  return pk
}

async function startChat(page: Page, peer: string, name: string) {
  await page.locator('button.new-btn').click()
  await expect(page.getByRole('heading', { name: 'Новый чат' })).toBeVisible({ timeout: 5000 })
  const userId = page.getByRole('textbox', { name: /User ID|Адрес друга/i })
  if (await userId.count()) await userId.fill(peer)
  else await page.locator('input').first().fill(peer)
  const nameBox = page.getByRole('textbox', { name: /Имя/i })
  if (await nameBox.count()) await nameBox.fill(name)
  await page.getByRole('button', { name: /Начать чат/i }).click()
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 15000 })
  await waitForWs(page)
}

test.describe('Alice → Bob DM', () => {
  test('message delivery via WS or REST reload', async ({ browser }) => {
    test.setTimeout(120000)
    const health = await fetch(`${API}/api/health`).then(r => r.status).catch(() => 0)
    expect(health, 'Go API required').toBe(200)

    const aliceCtx = await browser.newContext()
    await aliceCtx.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester1'))
    const bobCtx = await browser.newContext()
    await bobCtx.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester2'))
    const alice = await aliceCtx.newPage()
    const bob = await bobCtx.newPage()

    try {
      await boot(alice)
      await boot(bob)
      const alicePk = await pubkeyOf(alice)
      const bobPk = await pubkeyOf(bob)
      expect(alicePk).not.toBe(bobPk)

      await startChat(bob, alicePk, 'Alice')
      await startChat(alice, bobPk, 'Bob')
      await waitForWs(alice)
      await waitForWs(bob)

      const text = `E2E-DM-${Date.now()}`
      const box = alice.getByRole('textbox', { name: /Сообщение/i })
      await expect(box).toBeVisible({ timeout: 10000 })
      await box.fill(text)
      await box.press('Enter')

      await expect(alice.getByText(text).first()).toBeVisible({ timeout: 20000 })

      const bobSawWs = await bob.getByText(text).first().isVisible({ timeout: 20000 }).catch(() => false)
      if (!bobSawWs) {
        await bob.reload({ waitUntil: 'domcontentloaded' })
        await bob.waitForTimeout(4000)
        await bob.waitForFunction(() => !!(window as any).__proxiPubkey, null, { timeout: 25000 })
        const open = bob.locator('button:has-text("Alice")').first()
        if (await open.isVisible({ timeout: 5000 }).catch(() => false)) {
          await open.click()
          await bob.waitForTimeout(1000)
        }
      }
      await expect(bob.getByText(text).first()).toBeVisible({ timeout: 30000 })
    } finally {
      await aliceCtx.close().catch(() => {})
      await bobCtx.close().catch(() => {})
    }
  })
})