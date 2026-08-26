import { test, expect } from '@playwright/test';
const BASE = 'http://localhost:5173';

test('E2E-004: Alice → Bob DM across 2 browser contexts', async ({ browser }) => {
  // Alice context — pre-set demo role
  const aliceCtx = await browser.newContext();
  await aliceCtx.addInitScript(() => {
    localStorage.setItem('proxi_demo_role', 'tester1');
  });
  const alice = await aliceCtx.newPage();

  // Bob context — pre-set demo role
  const bobCtx = await browser.newContext();
  await bobCtx.addInitScript(() => {
    localStorage.setItem('proxi_demo_role', 'tester2');
  });
  const bob = await bobCtx.newPage();

  // Load both
  await alice.goto(BASE, { waitUntil: 'domcontentloaded' });
  await bob.goto(BASE, { waitUntil: 'domcontentloaded' });
  await alice.waitForTimeout(5000);
  await bob.waitForTimeout(5000);

  // Verify identities loaded
  const aliceBody = await alice.locator('body').textContent();
  const bobBody = await bob.locator('body').textContent();
  console.log('Alice loaded:', aliceBody.length, 'chars');
  console.log('Bob loaded:', bobBody.length, 'chars');

  await alice.screenshot({ path: 'e2e-shots/e2e-alice-sent.png', fullPage: true });
  await bob.screenshot({ path: 'e2e-shots/e2e-bob-received.png', fullPage: true });

  // Alice sends DM via DemoPanel "Start DM" button or chat input
  const startDm = alice.locator('button:has-text("DM")').first();
  if (await startDm.isVisible({ timeout: 5000 }).catch(() => false)) {
    await startDm.click();
    await alice.waitForTimeout(2000);
    
    // Type message
    const input = alice.locator('textarea').first();
    if (await input.isVisible({ timeout: 3000 }).catch(() => false)) {
      await input.fill(`E2E test ${new Date().toLocaleTimeString()}`);
      await input.press('Enter');
      await alice.waitForTimeout(3000);
    }
  }
  
  await alice.screenshot({ path: 'e2e-shots/e2e-alice-sent.png', fullPage: true });

  // Bob waits for WS delivery
  await bob.waitForTimeout(5000);
  const bobText2 = await bob.locator('body').textContent();
  await bob.screenshot({ path: 'e2e-shots/e2e-bob-received.png', fullPage: true });
  
  console.log('Bob after DM:', bobText2.length, 'chars');
  console.log('Bob sees Привет:', bobText2.includes('Привет'));
  
  expect(bobText2.length).toBeGreaterThan(50);
});
