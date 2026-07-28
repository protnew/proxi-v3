import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://localhost:5173'

async function waitForApp(page: Page) {
  await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('text=Indestructible Messenger', { timeout: 20000 })
  // identity/signup + ws
  await page.waitForTimeout(2500)
}

async function openNewChat(page: Page) {
  const btn = page.getByRole('button', { name: '✏️' })
  await btn.click()
  await expect(page.getByRole('heading', { name: 'Новый чат' })).toBeVisible({ timeout: 5000 })
}

async function startChat(page: Page, userId: string, name: string) {
  await openNewChat(page)
  // UI: textbox "User ID" + optional name (not old textarea pubkey)
  const idBox = page.getByRole('textbox', { name: /User ID|pubkey|Pubkey/i }).or(
    page.locator('input[placeholder*="User"], input[placeholder*="pubkey" i], input[placeholder*="ID"]').first()
  )
  // fallback: first textbox in dialog after heading
  if (await idBox.count() === 0) {
    await page.locator('heading:has-text("Новый чат")').locator('..').locator('input, textarea').first().fill(userId)
  } else {
    await idBox.first().fill(userId)
  }
  const nameBox = page.getByRole('textbox', { name: /Имя/i }).or(
    page.locator('input[placeholder*="Имя"]').first()
  )
  if (await nameBox.count()) {
    await nameBox.first().fill(name)
  }
  const start = page.getByRole('button', { name: /Начать чат/i })
  await expect(start).toBeEnabled({ timeout: 5000 })
  await start.click()
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 8000 })
}

async function sendText(page: Page, text: string) {
  const box = page.getByRole('textbox', { name: /Сообщение/i }).or(page.locator('textarea').first())
  await box.fill(text)
  await box.press('Enter')
  await page.waitForTimeout(400)
  await expect(page.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 8000 })
}

test.describe('Messenger UI (native)', () => {
  test('loads and shows title', async ({ page }) => {
    await page.goto(APP_URL)
    await expect(page.getByText('Indestructible Messenger')).toBeVisible({ timeout: 15000 })
  })

  test('opens new chat dialog with User ID field', async ({ page }) => {
    await waitForApp(page)
    await openNewChat(page)
    await expect(page.getByRole('button', { name: /Начать чат/i })).toBeVisible()
    // must NOT rely on old textarea placeholder
    const legacy = page.locator('textarea[placeholder="Вставь pubkey друга"]')
    expect(await legacy.count()).toBe(0)
  })

  test('creates chat and sends message', async ({ page }) => {
    await waitForApp(page)
    const name = `User${Date.now().toString().slice(-6)}`
    await startChat(page, 'a'.repeat(64), name)
    const msg = `e2e hello ${Date.now()}`
    await sendText(page, msg)
  })

  test('emoji picker opens', async ({ page }) => {
    await waitForApp(page)
    await startChat(page, 'b'.repeat(64), `Emoji${Date.now().toString().slice(-4)}`)
    await page.getByRole('button', { name: '😊' }).click()
    // picker visible somehow
    await page.waitForTimeout(300)
    const picker = page.locator('.emoji-picker, .eb').first()
    await expect(picker).toBeVisible({ timeout: 5000 })
  })
})
