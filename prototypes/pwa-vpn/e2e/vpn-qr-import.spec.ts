import { test, expect } from '@playwright/test'

/**
 * TZ-04-20260930 §3.5 — QR/import invite: paste a deterministic
 * `proxi+vpn://` invite → the panel accepts it and issues connect_invite
 * (hook: VPNProductPanel invite handler → lib/vpn.ts connectInviteVPN).
 *
 * Manual-run spec (NOT in CI): needs the dev server + core.
 *   npx playwright test e2e/vpn-qr-import.spec.ts --project=chromium
 */

const INVITE = [
  'proxi+vpn://',
  'onion=abcd1234.onion',            // deterministic fixture (no real donor)
  'token=fixed-token-1',
  'exp=4102444800',                   // 2100-01-01 — never expires in test
  'npub=npub1fixture',
  'sig=fixedsig',
].join('')

test('paste proxi+vpn:// invite → connect_invite attempt', async ({ page }) => {
  await page.goto('/?dev=1')

  // Open the VPN product panel (sidebar bottom entry) and the import field.
  await page.getByRole('button', { name: /vpn/i }).first().click()
  await page.getByRole('button', { name: /импорт|import|qr/i }).first().click()

  const input = page.locator('input[placeholder*="proxi+vpn"], textarea[placeholder*="proxi+vpn"]').first()
  await input.fill(INVITE)
  await page.getByRole('button', { name: /подключить|connect/i }).first().click()

  // The panel must move past «invite без endpoint/token» — either a real
  // connect attempt against the fixture (expected to fail dial) or an
  // explicit status. What we assert: NO silent drop of the invite.
  await expect
    .poll(async () => page.locator('[data-testid="vpn-status"], .vpn-status, [class*="statusText"]').first().textContent(),
      { timeout: 15_000 })
    .toBeTruthy()
  const status = await page.locator('[data-testid="vpn-status"], .vpn-status, [class*="statusText"]').first().textContent()
  // Fixture donor does not exist → the honest outcome is a dial error,
  // NOT «Инвайт без endpoint/token» (that would mean the parser failed).
  expect(status).not.toContain('без endpoint')
})

test('rejects invite without endpoint/token', async ({ page }) => {
  await page.goto('/?dev=1')
  await page.getByRole('button', { name: /vpn/i }).first().click()
  await page.getByRole('button', { name: /импорт|import|qr/i }).first().click()
  const input = page.locator('input[placeholder*="proxi+vpn"], textarea[placeholder*="proxi+vpn"]').first()
  await input.fill('proxi+vpn://nonsense')
  await page.getByRole('button', { name: /подключить|connect/i }).first().click()
  await expect(page.getByText(/без endpoint|без token/i).first()).toBeVisible({ timeout: 10_000 })
})
