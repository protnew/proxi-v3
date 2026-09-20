import { test, expect } from "@playwright/test"

const BASE = process.env.APP_URL || "http://127.0.0.1:5173"

test("1. app loads product VPN controls", async ({ page }) => {
  test.setTimeout(30000)
  await page.goto(BASE + "/?role=alice", { waitUntil: "domcontentloaded" })
  await page.waitForTimeout(4000)
  await expect(page.getByTestId("vpn-give")).toBeVisible({ timeout: 15000 })
})

test("2. Alice give one-click + Bob panel visible", async ({ browser }) => {
  test.setTimeout(60000)
  const aliceCtx = await browser.newContext()
  const bobCtx = await browser.newContext()
  const alice = await aliceCtx.newPage()
  const bob = await bobCtx.newPage()
  await bob.goto(BASE + "/?role=bob", { waitUntil: "domcontentloaded" })
  await alice.goto(BASE + "/?role=alice", { waitUntil: "domcontentloaded" })
  await alice.waitForTimeout(3500)
  await bob.waitForTimeout(3500)
  await alice.getByTestId("vpn-give").click({ timeout: 5000 })
  await alice.waitForTimeout(4000)
  const aBody = (await alice.locator("body").innerText()) + (await alice.getByTestId("vpn-log").innerText().catch(() => ""))
  expect(/Инвайт|invite|Раздаю VPN|sharing|Nostr|WebRTC/i.test(aBody)).toBeTruthy()
  await expect(bob.getByTestId("vpn-request")).toBeVisible()
  await aliceCtx.close()
  await bobCtx.close()
})
