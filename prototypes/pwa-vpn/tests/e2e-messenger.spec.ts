import { test, expect, Page } from '@playwright/test'

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:5173'

async function waitForApp(page: Page) {
  await page.goto(APP_URL, { waitUntil: 'domcontentloaded' })
  await page.waitForSelector('text=Indestructible Messenger', { timeout: 20000 })
  await page.waitForTimeout(2500)
}

async function openNewChat(page: Page) {
  await page.getByRole('button', { name: '✏️' }).click()
  await expect(page.getByRole('heading', { name: 'Новый чат' })).toBeVisible({ timeout: 5000 })
}

async function startChat(page: Page, userId: string, name: string) {
  await openNewChat(page)
  const idBox = page.getByRole('textbox', { name: /User ID|pubkey|Pubkey/i }).or(
    page.locator('input[placeholder*="User"], input[placeholder*="pubkey" i], input[placeholder*="ID"]').first(),
  )
  if (await idBox.count() === 0) {
    await page.locator('heading:has-text("Новый чат")').locator('..').locator('input, textarea').first().fill(userId)
  } else {
    await idBox.first().fill(userId)
  }
  const nameBox = page.getByRole('textbox', { name: /Имя/i }).or(page.locator('input[placeholder*="Имя"]').first())
  if (await nameBox.count()) await nameBox.first().fill(name)
  await page.getByRole('button', { name: /Начать чат/i }).click()
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 8000 })
}

async function sendText(page: Page, text: string) {
  const box = page.getByRole('textbox', { name: /Сообщение/i }).or(page.locator('textarea, input[placeholder*="Сообщен"]').first())
  await box.fill(text)
  await box.press('Enter')
  await page.waitForTimeout(500)
  await expect(page.locator('.msg-text', { hasText: text }).first()).toBeVisible({ timeout: 10000 })
}

test.describe('Messenger UI (tests/ orphan modernized)', () => {
  test('loads title', async ({ page }) => {
    await page.goto(APP_URL)
    await expect(page.getByText('Indestructible Messenger')).toBeVisible({ timeout: 15000 })
  })

  test('new chat dialog', async ({ page }) => {
    await waitForApp(page)
    await openNewChat(page)
    await expect(page.getByRole('button', { name: /Начать чат/i })).toBeVisible()
  })

  test('create chat + send', async ({ page }) => {
    await waitForApp(page)
    const name = `Orph${Date.now().toString().slice(-5)}`
    await startChat(page, 'c'.repeat(64), name)
    await sendText(page, `orphan msg ${Date.now()}`)
  })

  test('emoji picker', async ({ page }) => {
    await waitForApp(page)
    await startChat(page, 'd'.repeat(64), `Emo${Date.now().toString().slice(-4)}`)
    await page.getByRole('button', { name: '😊' }).click()
    await expect(page.locator('.emoji-picker, .eb').first()).toBeVisible({ timeout: 5000 })
  })
})
