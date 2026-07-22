import { test, expect } from '@playwright/test'

const APP_URL = 'http://localhost:5173'

async function waitForApp(page: any) {
  await page.goto(APP_URL)
  await page.waitForSelector('.app', { timeout: 15000 })
  await page.waitForTimeout(3000) // wait for relay connections
}

test.describe('Messenger UI', () => {
  test('loads and shows empty state', async ({ page }) => {
    await page.goto(APP_URL)
    await page.waitForSelector('.app', { timeout: 15000 })
    // Should show "Indestructible Messenger" text
    await expect(page.locator('text=Indestructible Messenger')).toBeVisible({ timeout: 5000 })
  })

  test('can open new chat dialog', async ({ page }) => {
    await waitForApp(page)
    await page.click('button:has-text("✏️")')
    await expect(page.locator('text=Новый чат')).toBeVisible()
    await expect(page.locator('text=Public Key')).toBeVisible()
  })

  test('can open settings and show pubkey', async ({ page }) => {
    await waitForApp(page)
    await page.click('button:has-text("☰")')
    await expect(page.locator('text=Настройки')).toBeVisible()
    // Public Key should be displayed
    await expect(page.locator('.kr code')).toBeVisible()
  })

  test('can create a chat with a fake pubkey', async ({ page }) => {
    await waitForApp(page)
    await page.click('button:has-text("✏️")')
    await page.fill('textarea[placeholder="Вставь pubkey друга"]', 'a'.repeat(64))
    await page.fill('input[placeholder="Имя друга"]', 'TestUser')
    await page.click('button:has-text("💬 Начать чат")')
    await page.waitForTimeout(500)
    // Chat should appear in sidebar
    await expect(page.locator('text=TestUser')).toBeVisible()
  })

  test('emoji picker opens and inserts emoji', async ({ page }) => {
    // Create a chat first
    await waitForApp(page)
    await page.click('button:has-text("✏️")')
    await page.fill('textarea[placeholder="Вставь pubkey друга"]', 'b'.repeat(64))
    await page.fill('input[placeholder="Имя друга"]', 'EmojiTest')
    await page.click('button:has-text("💬 Начать чат")')
    await page.waitForTimeout(500)
    // Click emoji button
    await page.click('button:has-text("😊")')
    await expect(page.locator('.emoji-picker')).toBeVisible()
    // Click first emoji
    await page.locator('.eb').first().click()
    // Textarea should have emoji
    const text = await page.inputValue('textarea')
    expect(text.length).toBeGreaterThan(0)
  })

  test('right-click shows context menu', async ({ page }) => {
    // Create chat + send message first
    await waitForApp(page)
    await page.click('button:has-text("✏️")')
    await page.fill('textarea[placeholder="Вставь pubkey друга"]', 'c'.repeat(64))
    await page.fill('input[placeholder="Имя друга"]', 'ContextTest')
    await page.click('button:has-text("💬 Начать чат")')
    await page.waitForTimeout(300)
    // Send a message
    await page.fill('textarea', 'Test message')
    await page.press('textarea', 'Enter')
    await page.waitForTimeout(300)
    // Right-click on message
    const bubble = page.locator('.bubble').first()
    await bubble.click({ button: 'right' })
    await expect(page.locator('text=↩ Ответить')).toBeVisible()
  })
})

test.describe('DM between two contexts', () => {
  test('Alice sends DM — message appears in her chat', async ({ browser }) => {
    const aliceCtx = await browser.newContext()
    const alice = await aliceCtx.newPage()

    try {
      await waitForApp(alice)

      // Get Alice's pubkey from settings
      await alice.click('button:has-text("☰")')
      const codeEl = alice.locator('.kr code')
      const pkText = await codeEl.textContent()
      expect(pkText!.length).toBeGreaterThan(10)
      await alice.click('button:has-text("←")')

      // Start chat with fake key
      await alice.click('button:has-text("✏️")')
      await alice.fill('textarea[placeholder="Вставь pubkey друга"]', 'd'.repeat(64))
      await alice.fill('input[placeholder="Имя друга"]', 'Bob')
      await alice.click('button:has-text("💬 Начать чат")')
      await alice.waitForTimeout(300)

      // Send message
      const testMsg = `Hello from Alice ${Date.now()}`
      await alice.fill('textarea', testMsg)
      await alice.press('textarea', 'Enter')
      await alice.waitForTimeout(500)

      // Verify message appears
      const msgTexts = await alice.locator('.msg-text').allTextContents()
      expect(msgTexts).toContain(testMsg)
    } finally {
      await aliceCtx.close()
    }
  })
})
