/**
 * WebRTC E2E: Alice invite one-click + authenticated API probes on :8090.
 * No AmneziaVPN GUI. No /api/vpn/amnezia/import.
 */
import { test, expect } from "@playwright/test"
import { apiToken } from './helpers/auth'

const APP = process.env.APP_URL || "http://127.0.0.1:5173"
const API = process.env.API_URL || "http://127.0.0.1:8090"

function auth(token: string) {
  return { Authorization: `Bearer ${token}` }
}

test("Alice gives VPN: one-click demo partner + STUN badge", async ({ page }) => {
  await page.goto(`${APP}/?role=alice`, { waitUntil: "domcontentloaded", timeout: 15000 })
  await page.waitForTimeout(3500)

  await expect(page.locator("[data-testid=vpn-give]")).toBeVisible({ timeout: 15000 })
  const bodyBefore = await page.locator("body").innerText()
  expect(/STUN|ICE|WebRTC|VPN/i.test(bodyBefore)).toBeTruthy()

  await page.locator("[data-testid=vpn-give]").click()
  const friend = page.locator("[data-testid=vpn-friend-id]")
  if (await friend.isVisible({ timeout: 1500 }).catch(() => false)) {
    // Real x-only pubkey of demo secret '2'*64 (P1: fake pubkeys fail auth)
    await friend.fill("466d7fcae563e5cb09a0d1870bb580344804617879a14949cf22285f1bae3f27")
    await page.locator("[data-testid=vpn-share-confirm]").click()
  }
  await page.waitForTimeout(3500)

  const log = (await page.locator("[data-testid=vpn-log]").textContent().catch(() => "")) || ""
  const body = await page.locator("body").innerText()
  const ok = /Инвайт|invite|Раздаю VPN|WebRTC|exit node|Nostr|sharing|STUN|ICE/i.test(log + body)
  expect(ok).toBeTruthy()
})

test("TURN config endpoint STUN-only", async ({ request }) => {
  const token = await apiToken(request)
  const r = await request.get(`${API}/api/vpn/turn/config`, { headers: auth(token) })
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.status === "not_configured" || body.urls).toBeTruthy()
})

test("AmneziaWG config: desktop_only phase", async ({ request }) => {
  const token = await apiToken(request)
  const r = await request.get(`${API}/api/vpn/amnezia`, { headers: auth(token) })
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.phase).toBe("desktop_only")
  expect(body.junkPacketCount).toBeGreaterThan(0)
})

test("In-app tunnel API up/userspace", async ({ request }) => {
  const token = await apiToken(request)
  const start = await request.post(`${API}/api/vpn/amnezia/tunnel`, {
    headers: auth(token),
    data: {
      peerPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      endpoint: "127.0.0.1:51820",
    },
  })
  expect(start.ok()).toBeTruthy()
  const j = await start.json()
  expect(["up", "conf_ready"]).toContain(j.state)
  await request.delete(`${API}/api/vpn/amnezia/tunnel`, { headers: auth(token) })
})

test("libp2p config endpoint: desktop_only phase", async ({ request }) => {
  const token = await apiToken(request)
  const r = await request.get(`${API}/api/vpn/libp2p`, { headers: auth(token) })
  expect(r.ok()).toBeTruthy()
  const body = await r.json()
  expect(body.phase).toBe("desktop_only")
  expect(String(body.table || "")).toContain("57")
})
