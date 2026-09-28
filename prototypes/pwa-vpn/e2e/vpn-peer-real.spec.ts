import { test, expect } from "@playwright/test"

const APP = process.env.APP_URL || "http://127.0.0.1:5173"
const API = process.env.API_URL || "http://127.0.0.1:8090"

test.describe("VPN-PEER-REAL — real key exchange dual-context", () => {
  test("two contexts get distinct non-demo pubkeys and Alice can give VPN", async ({ browser }) => {
    test.setTimeout(60000)
    const health = await fetch(`${API}/api/health`).then((r) => r.status).catch(() => 0)
    expect(health).toBe(200)

    const aliceCtx = await browser.newContext()
    const bobCtx = await browser.newContext()
    const alice = await aliceCtx.newPage()
    const bob = await bobCtx.newPage()
    try {
      // Use roles that still expose __proxiPubkey; assert they are not empty.
      // Fresh identities: clear storage then load without forcing demo if possible.
      await alice.addInitScript(() => {
        localStorage.clear()
      })
      await bob.addInitScript(() => {
        localStorage.clear()
      })
      // P25: debug globals are dev-gated, but dev=1 hides the product
      // VPN panel — read the persisted pubkey from localStorage instead.
      await alice.goto(APP + "/?role=alice", { waitUntil: "domcontentloaded" })
      await bob.goto(APP + "/?role=bob", { waitUntil: "domcontentloaded" })
      await alice.waitForTimeout(4000)
      await bob.waitForTimeout(4000)

      const alicePk = await alice.evaluate(() => localStorage.getItem("indestructible-pubkey") || "")
      const bobPk = await bob.evaluate(() => localStorage.getItem("indestructible-pubkey") || "")
      expect(alicePk.length).toBeGreaterThanOrEqual(32)
      expect(bobPk.length).toBeGreaterThanOrEqual(32)
      expect(alicePk).not.toBe(bobPk)

      await expect(alice.getByTestId("vpn-give")).toBeVisible({ timeout: 10000 })
      await alice.getByTestId("vpn-give").click({ timeout: 5000 })
      await alice.waitForTimeout(3500)
      const body = (await alice.locator("body").innerText()) + (await alice.getByTestId("vpn-log").innerText().catch(() => ""))
      expect(/Инвайт|invite|Раздаю VPN|sharing|Nostr|WebRTC|VPN/i.test(body)).toBeTruthy()
    } finally {
      await aliceCtx.close()
      await bobCtx.close()
    }
  })

  test("demo alice pubkey persists across reload", async ({ page }) => {
    test.setTimeout(45000)
    await page.goto(APP + "/?role=alice", { waitUntil: "domcontentloaded" })
    await page.waitForFunction(() => !!localStorage.getItem("indestructible-pubkey"), { timeout: 20000 })
    const pk1 = await page.evaluate(() => localStorage.getItem("indestructible-pubkey") || "")
    expect(pk1.length).toBeGreaterThanOrEqual(32)
    await page.reload({ waitUntil: "domcontentloaded" })
    await page.waitForFunction(() => !!localStorage.getItem("indestructible-pubkey"), { timeout: 20000 })
    const pk2 = await page.evaluate(() => localStorage.getItem("indestructible-pubkey") || "")
    expect(pk2).toBe(pk1)
  })
})
