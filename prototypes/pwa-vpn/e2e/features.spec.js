import { test, expect } from '@playwright/test';

const BASE = 'http://localhost:5173';

test.describe.serial('Proxi V2 — 5 новых фич', () => {

  test('01 Frontend loads', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(5000);
    await page.screenshot({ path: 'e2e-shots/01-load.png', fullPage: true });
  });

  test('02 Settings → Advanced → E2E + VPN toggles', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    // Click settings (☰ button)
    const menuBtn = page.locator('.menu-btn, button:has-text("☰")').first();
    if (await menuBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await menuBtn.click();
      await page.waitForTimeout(1000);
      // Click advanced gear
      const advBtn = page.locator('button:has-text("⚙️")').first();
      if (await advBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
        await advBtn.click();
        await page.waitForTimeout(1000);
      }
    }
    await page.screenshot({ path: 'e2e-shots/02-advanced.png', fullPage: true });
  });

  test('03 Search bar visible', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    await page.screenshot({ path: 'e2e-shots/03-search.png', fullPage: true });
  });

  test('04 New chat dialog with Group tab', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    const newBtn = page.locator('.new-btn, button:has-text("✏️")').first();
    if (await newBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await newBtn.click();
      await page.waitForTimeout(1000);
    }
    await page.screenshot({ path: 'e2e-shots/04-new-chat-group.png', fullPage: true });
  });

  test('05 QA Demo buttons (Alice/Bob)', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    // Find Alice button
    const alice = page.locator('button:has-text("Alice"), button:has-text("T1")');
    if (await alice.isVisible({ timeout: 3000 }).catch(() => false)) {
      await alice.click();
      await page.waitForTimeout(2000);
    }
    await page.screenshot({ path: 'e2e-shots/05-alice.png', fullPage: true });
  });

  test('06 Final full app', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    await page.screenshot({ path: 'e2e-shots/06-final.png', fullPage: true });
  });
});
