import { test, expect } from "@playwright/test"

const BASE = process.env.APP_URL || "http://127.0.0.1:5173"

test.setTimeout(60000)

test("Alice invite → Bob UI alive (one-click product path)", async ({ browser }) => {
  const aliceCtx = await browser.newContext()
  const bobCtx = await browser.newContext()
  const alice = await aliceCtx.newPage()
  const bob = await bobCtx.newPage()
  await bob.goto(BASE + "/?role=bob", { waitUntil: "domcontentloaded" })
  await alice.goto(BASE + "/?role=alice", { waitUntil: "domcontentloaded" })
  await alice.waitForTimeout(3500)
  await bob.waitForTimeout(3500)

  await expect(alice.getByTestId("vpn-give")).toBeVisible({ timeout: 15000 })
  await alice.getByTestId("vpn-give").click({ timeout: 5000 })
  await alice.waitForTimeout(4000)
  const body = (await alice.locator("body").innerText()) + (await alice.getByTestId("vpn-log").innerText().catch(() => ""))
  expect(/Инвайт|invite|Раздаю VPN|sharing|Nostr|WebRTC/i.test(body)).toBeTruthy()
  await expect(bob.getByTestId("vpn-request")).toBeVisible({ timeout: 10000 })

  await aliceCtx.close()
  await bobCtx.close()
})
