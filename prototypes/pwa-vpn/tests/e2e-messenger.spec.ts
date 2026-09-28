import { test, expect, Page } from "@playwright/test"

const APP_URL = process.env.APP_URL || "http://127.0.0.1:5173"

async function waitForApp(page: Page) {
  await page.addInitScript(() => localStorage.setItem("proxi_demo_role", "tester1"))
  await page.goto(APP_URL + "?role=alice", { waitUntil: "domcontentloaded" })
  const brand = page.getByTestId("chat-empty-brand").or(page.getByRole("heading", { name: "Proxi" }))
  const newBtn = page.locator("button.new-btn")
  await expect(brand.or(newBtn).first()).toBeVisible({ timeout: 20000 })
  await page.waitForTimeout(1500)
}

async function openNewChat(page: Page) {
  await page.locator("button.new-btn").click()
  await expect(page.getByRole("heading", { name: "Новый чат" })).toBeVisible({ timeout: 5000 })
}

async function startChat(page: Page, userId: string, name: string) {
  await openNewChat(page)
  const idBox = page.getByRole("textbox", { name: /User ID|pubkey|Pubkey|Адрес друга/i }).or(
    page.locator('input[placeholder*="User"], input[placeholder*="pubkey" i], input[placeholder*="ID"]').first(),
  )
  if ((await idBox.count()) === 0) {
    await page.getByRole("heading", { name: "Новый чат" }).locator("..").locator("input, textarea").first().fill(userId)
  } else {
    await idBox.first().fill(userId)
  }
  const nameBox = page.getByRole("textbox", { name: /Имя/i }).or(page.locator('input[placeholder*="Имя"]').first())
  if (await nameBox.count()) await nameBox.first().fill(name)
  const start = page.getByRole("button", { name: /Начать чат/i })
  await expect(start).toBeEnabled({ timeout: 5000 })
  await start.click()
  await expect(page.getByText(name).first()).toBeVisible({ timeout: 8000 })
}

async function sendText(page: Page, text: string) {
  const box = page.getByRole("textbox", { name: /Сообщение/i }).or(page.locator("textarea").first())
  await box.fill(text)
  await box.press("Enter")
  await page.waitForTimeout(400)
  await expect(page.getByText(text).first()).toBeVisible({ timeout: 8000 })
}

test.describe("Messenger UI (tests/ orphan modernized)", () => {
  test("loads title", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("proxi_demo_role", "tester1"))
    await page.goto(APP_URL + "?role=alice")
    await expect(
      page.getByTestId("chat-empty-brand").or(page.getByRole("heading", { name: "Proxi" })).first(),
    ).toBeVisible({ timeout: 15000 })
  })

  test("new chat dialog", async ({ page }) => {
    await waitForApp(page)
    await openNewChat(page)
    await expect(page.getByRole("button", { name: /Начать чат/i })).toBeVisible()
  })

  test("create chat + send", async ({ page }) => {
    await waitForApp(page)
    const name = `Orph${Date.now().toString().slice(-5)}`
    await startChat(page, "c".repeat(64), name)
    await sendText(page, `orphan msg ${Date.now()}`)
  })

  test("emoji picker", async ({ page }) => {
    await waitForApp(page)
    await startChat(page, "d".repeat(64), `Emo${Date.now().toString().slice(-4)}`)
    await page.getByRole("button", { name: "Смайлики" }).click()
    await expect(page.locator(".emoji-picker, .eb").first()).toBeVisible({ timeout: 5000 })
  })
})
