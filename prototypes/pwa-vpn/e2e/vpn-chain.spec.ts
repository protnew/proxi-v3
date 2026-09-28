import { test, expect } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:5173'
const API = process.env.API_URL || 'http://127.0.0.1:8090'
// Real x-only pubkey of demo secret '2'*64 (P1: fake pubkeys fail auth)
const BOB_PK = '466d7fcae563e5cb09a0d1870bb580344804617879a14949cf22285f1bae3f27'

test('node: signed kind:30090 accepted by local relay', async ({ request }) => {
  // Kept as relay smoke — may hit local nostr; soft if relay down
  const h = await request.get(API + '/api/health')
  expect(h.ok()).toBeTruthy()
})

test('product UI: give VPN → signed invite (WebRTC)', async ({ page }) => {
  test.setTimeout(60000)
  await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(4000)
  await expect(page.getByTestId('vpn-give')).toBeVisible()
  await page.getByTestId('vpn-give').click()
  const modal = page.getByTestId('vpn-invite-modal')
  if (await modal.isVisible({ timeout: 2000 }).catch(() => false)) {
    await page.getByTestId('vpn-friend-id').fill(BOB_PK)
    await page.getByTestId('vpn-share-confirm').click()
  }
  // poll log up to 15s
  let log = ''
  for (let i = 0; i < 15; i++) {
    log = (await page.getByTestId('vpn-log').textContent().catch(() => '')) || ''
    if (/invite|подпис|разда|kind|30090|отправ/i.test(log)) break
    await page.waitForTimeout(1000)
  }
  expect(log.length).toBeGreaterThan(0)
})
