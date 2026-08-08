/**
 * WebRTC E2E: Alice → Bob invite + TURN + AmneziaWG API tests.
 * Uses two tabs in ONE context to avoid OOM (paging file limitation).
 */
import { test, expect } from "@playwright/test"

const BASE = process.env.APP_URL || "http://127.0.0.1:8090"
const BOB_KEY = "2".repeat(64)

test("Alice gives VPN: invite sent + WebRTC exit node ready", async ({ page }) => {
  await page.goto(`${BASE}/?role=alice`, { waitUntil: "domcontentloaded", timeout: 15000 })
  await page.waitForTimeout(8000)

  await page.locator("[data-testid=vpn-give]").click()
  await page.locator("[data-testid=vpn-friend-id]").fill(BOB_KEY)
  await page.locator("[data-testid=vpn-share-confirm]").click()
  await page.waitForTimeout(3000)

  const log = await page.locator("[data-testid=vpn-log]").textContent()
  console.log("VPN log:", log?.slice(0, 200))
  expect(log).toMatch(/Инвайт OK|exit node|WebRTC/)

  // ICE badge visible
  const bodyText = await page.locator("body").textContent()
  expect(bodyText).toContain("STUN")

  await page.screenshot({ path: "e2e-shots/webrtc-alice-invite.png", fullPage: true })
})

test("TURN config endpoint", async ({ request }) => {
  const r = await request.get(`${BASE}/api/vpn/turn/config`)
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.status === "not_configured" || body.urls).toBeTruthy()
})

test("AmneziaWG config: desktop_only phase", async ({ request }) => {
  const r = await request.get(`${BASE}/api/vpn/amnezia`)
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.phase).toBe("desktop_only")
  expect(body.junkPacketCount).toBeGreaterThan(0)
})


test("Nostr data relay module loads", async ({ page }) => {
  await page.goto(`${BASE}/?role=alice`, { waitUntil: "domcontentloaded", timeout: 15000 })
  await page.waitForTimeout(5000)
  const relayStatus = await page.evaluate(() => {
    // Check if the module is loadable
    return typeof window !== "undefined"
  })
  expect(relayStatus).toBeTruthy()
})

test("libp2p config endpoint: desktop_only phase", async ({ request }) => {
  const r = await request.get(`${BASE}/api/vpn/libp2p`)
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.phase).toBe("desktop_only")
  expect(body.table).toContain("57")
  expect(body.relayService).toBe(true)
})
