/**
 * MSG-101: Alice → Bob DM MUST arrive without Bob reload (WebSocket realtime).
 */
import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:8090'

async function waitForApp(page: Page) {
  await page.goto(APP_URL, { waitUntil: 'domcontentloaded' }) // role set by callers
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
  const userId = page.getByRole('textbox', { name: /User ID|Адрес друга/i })
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

test.describe('MSG-101 WS no-reload', () => {
  test('Bob sees Alice message without reload', async ({ browser }) => {
    test.setTimeout(90000)
    const aliceCtx = await browser.newContext()
    const bobCtx = await browser.newContext()
    const alice = await aliceCtx.newPage()
    const bob = await bobCtx.newPage()

    await alice.goto(APP_URL + '?role=alice', { waitUntil: 'domcontentloaded' })
    await bob.goto(APP_URL + '?role=bob', { waitUntil: 'domcontentloaded' })
    await waitForApp(alice)
    await waitForApp(bob)
    const alicePk = await pubkeyOf(alice)
    const bobPk = await pubkeyOf(bob)
    expect(alicePk).not.toBe(bobPk)

    await startChat(bob, alicePk, 'Alice')
    await startChat(alice, bobPk, 'Bob')

    // Capture bob navigation to detect accidental reload
    let bobReloaded = false
    bob.on('framenavigated', (frame) => {
      if (frame === bob.mainFrame()) bobReloaded = true
    })
    // Reset flag after chat setup navigations settled
    bobReloaded = false

    const text = `MSG101-WS-${Date.now()}`
    const box = alice.getByRole('textbox', { name: /Сообщение/i }).or(alice.locator('textarea').first())
    await box.fill(text)
    await box.press('Enter')

    await expect(alice.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 15000 })
    // STRICT: Bob without reload
    await expect(bob.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 20000 })
    expect(bobReloaded, 'Bob must not reload').toBe(false)

    await alice.screenshot({ path: 'test-results/msg101-alice.png', fullPage: true })
    await bob.screenshot({ path: 'test-results/msg101-bob.png', fullPage: true })

    await aliceCtx.close()
    await bobCtx.close()
  })
})
