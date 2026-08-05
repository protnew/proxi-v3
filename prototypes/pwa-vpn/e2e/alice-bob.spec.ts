import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://localhost:5173'
const API = process.env.API_URL || 'http://localhost:8080'

async function waitForApp(page: Page) {
  await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })
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
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 15000 })
}

async function openChatByName(page: Page, name: string) {
  // Click on chat in sidebar by name
  const chatBtn = page.locator('button', { hasText: name }).first()
  if (await chatBtn.count()) {
    await chatBtn.click()
    await page.waitForTimeout(500)
  }
}

test.describe('Alice → Bob DM', () => {
  test('message delivery via WS or REST reload', async ({ browser }) => {
    test.setTimeout(90000) // 90s for two-browser E2E
    const health = await fetch(`${API}/api/health`).then(r => r.status).catch(() => 0)
    expect(health, 'Go server :8080 required').toBe(200)

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

      const text = `E2E-DM-${Date.now()}`
      const box = alice.getByRole('textbox', { name: /Сообщение/i }).or(alice.locator('textarea').first())
      await box.fill(text)
      await box.press('Enter')

      // Alice sees her message
      await expect(alice.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 15000 })

      // Bob: try WS real-time first (8s)
      let delivered = false
      try {
        await expect(bob.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 15000 })
        delivered = true
      } catch {
        // WS didn't deliver — use REST fallback
      }

      if (!delivered) {
        // Reload Bob, open the chat, check REST persistence
        await bob.reload({ waitUntil: 'domcontentloaded' })
        await waitForApp(bob)
        await openChatByName(bob, 'Bob')
        // Wait for messages to load
        await bob.waitForTimeout(2000)
        // Check if message appears in chat history
        const msgCount = await bob.locator('.msg-text', { hasText: text }).count()
        if (msgCount === 0) {
          // Final attempt: open the Alice chat
          await openChatByName(bob, 'Alice')
          await bob.waitForTimeout(2000)
        }
        await expect(bob.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 20000 })
      }
    } finally {
      await aliceCtx.close()
      await bobCtx.close()
    }
  })
})

