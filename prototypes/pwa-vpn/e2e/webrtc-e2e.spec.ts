/**
 * WebRTC E2E: Alice invite one-click (demo partner) + API probes.
 * Product UI auto-fills Bob when ?role=alice — no modal paste required.
 */
import { test, expect } from "@playwright/test"

const BASE = process.env.APP_URL || "http://127.0.0.1:8090"

test("Alice gives VPN: one-click demo partner + STUN badge", async ({ page }) => {
  await page.goto(`${BASE}/?role=alice`, { waitUntil: "domcontentloaded", timeout: 15000 })
  await page.waitForTimeout(3500)

  await expect(page.locator("[data-testid=vpn-give]")).toBeVisible()
  // Demo partner line may appear
  const bodyBefore = await page.locator("body").innerText()
  expect(bodyBefore).toMatch(/STUN/i)

  await page.locator("[data-testid=vpn-give]").click()
  // If modal still opens, fill and confirm; else one-click path
  const friend = page.locator("[data-testid=vpn-friend-id]")
  if (await friend.isVisible({ timeout: 1500 }).catch(() => false)) {
    await friend.fill("2".repeat(64))
    await page.locator("[data-testid=vpn-share-confirm]").click()
  }
  await page.waitForTimeout(3500)

  const log = (await page.locator("[data-testid=vpn-log]").textContent().catch(() => "")) || ""
  const body = await page.locator("body").innerText()
  const ok = /Инвайт|invite|Раздаю VPN|WebRTC|exit node|Nostr|sharing/i.test(log + body)
  expect(ok).toBeTruthy()
  expect(body).toMatch(/STUN/i)

  await page.screenshot({ path: "e2e-shots/webrtc-alice-invite.png", fullPage: true })
})

test("TURN config endpoint STUN-only", async ({ request }) => {
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

test("In-app tunnel API up/userspace", async ({ request }) => {
  const start = await request.post(`${BASE}/api/vpn/amnezia/tunnel`, {
    data: {
      peerPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      endpoint: "127.0.0.1:51820",
    },
  })
  expect(start.ok()).toBeTruthy()
  const j = await start.json()
  expect(["up", "conf_ready"]).toContain(j.state)
  await request.delete(`${BASE}/api/vpn/amnezia/tunnel`)
})

test("libp2p config endpoint: desktop_only phase", async ({ request }) => {
  const r = await request.get(`${BASE}/api/vpn/libp2p`)
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.phase).toBe("desktop_only")
  expect(String(body.table || "")).toContain("57")
})
