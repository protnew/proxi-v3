import { test, expect } from '@playwright/test';
const BASE = process.env.APP_URL || 'http://localhost:5173';

test.describe.serial('10 Functional Screenshots', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('proxi_demo_role', 'tester1');
    });
  });

  test('01 Main chat screen', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    await page.screenshot({ path: 'e2e-shots/s01-main.png', fullPage: true });
    const text = await page.locator('body').textContent();
    expect(text.length).toBeGreaterThan(20);
  });

  test('02 Sidebar search', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const search = page.locator('input[placeholder*="\u041F\u043E\u0438\u0441\u043A"], input[type="text"]').first();
    await search.fill('test query');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'e2e-shots/s02-search.png', fullPage: true });
    const val = await search.inputValue();
    expect(val).toBe('test query');
  });

  test('03 New chat dialog', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const btn = page.locator('button:has-text("\u270F\uFE0F")').first();
    await btn.click();
    await page.waitForTimeout(1000);
    await page.screenshot({ path: 'e2e-shots/s03-newchat.png', fullPage: true });
    expect(await page.locator('body').textContent()).toBeTruthy();
  });

  test('04 Group tab', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    await page.locator('button:has-text("\u270F\uFE0F")').first().click();
    await page.waitForTimeout(800);
    const groupTab = page.locator('button:has-text("\u0413\u0440\u0443\u043F\u043F\u0430")');
    if (await groupTab.isVisible({ timeout: 3000 }).catch(() => false)) {
      await groupTab.click();
      await page.waitForTimeout(500);
    }
    await page.screenshot({ path: 'e2e-shots/s04-group.png', fullPage: true });
  });

  test('05 Settings profile', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    await page.locator('button:has-text("\u2630")').first().click();
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'e2e-shots/s05-profile.png', fullPage: true });
    expect(await page.locator('body').textContent()).toBeTruthy();
  });

  test('06 Public key + QR', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    await page.locator('button:has-text("\u2630")').first().click();
    await page.waitForTimeout(800);
    const qr = page.locator('button:has-text("QR")');
    if (await qr.isVisible({ timeout: 2000 }).catch(() => false)) {
      await qr.click();
      await page.waitForTimeout(500);
    }
    await page.screenshot({ path: 'e2e-shots/s06-qr.png', fullPage: true });
  });

  test('07 Advanced tab', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    await page.locator('button:has-text("\u2630")').first().click();
    await page.waitForTimeout(800);
    const adv = page.locator('button:has-text("\u2699"), button:has-text("\u0414\u043E\u043F\u043E\u043B\u043D\u0438\u0442\u0435\u043B\u044C\u043D\u043E")');
    if (await adv.first().isVisible({ timeout: 2000 }).catch(() => false)) {
      await adv.first().click();
      await page.waitForTimeout(500);
    }
    await page.screenshot({ path: 'e2e-shots/s07-advanced.png', fullPage: true });
  });

  test('08 E2E toggle click', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    await page.locator('button:has-text("\u2630")').first().click();
    await page.waitForTimeout(800);
    const adv = page.locator('button:has-text("\u2699"), button:has-text("\u0414\u043E\u043F\u043E\u043B\u043D\u0438\u0442\u0435\u043B\u044C\u043D\u043E")');
    if (await adv.first().isVisible({ timeout: 2000 }).catch(() => false)) {
      await adv.first().click();
      await page.waitForTimeout(500);
    }
    const tog = page.locator('.toggle, [class*="toggle"], [role="switch"]');
    if (await tog.first().isVisible({ timeout: 2000 }).catch(() => false)) {
      await tog.first().click();
      await page.waitForTimeout(300);
    }
    await page.screenshot({ path: 'e2e-shots/s08-e2e-clicked.png', fullPage: true });
  });

  test('09 Alice demo', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    const alice = page.locator('button:has-text("Alice")');
    if (await alice.first().isVisible({ timeout: 3000 }).catch(() => false)) {
      await alice.first().click();
      await page.waitForTimeout(2000);
    }
    await page.screenshot({ path: 'e2e-shots/s09-alice.png', fullPage: true });
  });

  test('10 Bob demo final', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(4000);
    const bob = page.locator('button:has-text("Bob")');
    if (await bob.first().isVisible({ timeout: 3000 }).catch(() => false)) {
      await bob.first().click();
      await page.waitForTimeout(2000);
    }
    await page.screenshot({ path: 'e2e-shots/s10-bob-final.png', fullPage: true });
    const text = await page.locator('body').textContent();
    expect(text.length).toBeGreaterThan(20);
  });
});
