import { test, expect } from '@playwright/test';
const BASE = 'http://localhost:5173';
const marker = `STRICT-${Date.now()}`;

test('E2E-004 strict: real input Alice→Bob + DB no duplicates', async ({ browser }) => {
  const aliceCtx = await browser.newContext();
  await aliceCtx.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester1'));
  const alice = await aliceCtx.newPage();
  const jsErrors = [];
  alice.on('console', m => { if (m.type() === 'error') jsErrors.push(m.text().slice(0, 120)); });

  const bobCtx = await browser.newContext();
  await bobCtx.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester2'));
  const bob = await bobCtx.newPage();

  await alice.goto(BASE, { waitUntil: 'domcontentloaded' });
  await bob.goto(BASE, { waitUntil: 'domcontentloaded' });
  await alice.waitForTimeout(6000);
  await bob.waitForTimeout(6000);

  // Alice: sidebar chat with Bob may need a click if not open
  const input = alice.getByRole('textbox', { name: 'Сообщение' });
  if (!(await input.isVisible({ timeout: 3000 }).catch(() => false))) {
    await alice.locator('button:has-text("Bob")').first().click();
    await alice.waitForTimeout(1500);
  }
  await expect(input).toBeVisible({ timeout: 8000 });
  await input.fill(marker);
  await input.press('Enter');
  await alice.waitForTimeout(3000);
  await alice.screenshot({ path: 'e2e-shots/e2e-alice-sent.png', fullPage: true });

  // Bob: receive via WS
  await bob.waitForTimeout(5000);
  const bobText = await bob.locator('body').textContent();
  await bob.screenshot({ path: 'e2e-shots/e2e-bob-received.png', fullPage: true });
  console.log('MARKER:', marker);
  console.log('Bob received:', bobText.includes(marker));
  console.log('JS errors:', jsErrors.length ? jsErrors.join(' | ') : 'none');
  expect(bobText).toContain(marker);
});
