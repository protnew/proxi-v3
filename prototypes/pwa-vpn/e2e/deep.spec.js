import { test, expect } from '@playwright/test';
const BASE = process.env.APP_URL || 'http://127.0.0.1:8080';

test.describe.serial('10 Functional Screenshots', () => {

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
    const search = page.locator('input[placeholder*="Поиск"], input[type="text"]').first();
    await search.fill('test query');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'e2e-shots/s02-search.png', fullPage: true });
  });

  test('03 New chat dialog', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const btn = page.locator('.new-btn').first();
    if (await btn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await btn.click();
      await page.waitForTimeout(1000);
    }
    await page.screenshot({ path: 'e2e-shots/s03-newchat.png', fullPage: true });
  });

  test('04 Group tab', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const btn = page.locator('.new-btn').first();
    if (await btn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await btn.click();
      await page.waitForTimeout(800);
      const grp = page.locator('button:has-text("Группа")');
      if (await grp.isVisible({ timeout: 1000 }).catch(() => false)) await grp.click();
      await page.waitForTimeout(500);
    }
    await page.screenshot({ path: 'e2e-shots/s04-group.png', fullPage: true });
  });

  test('05 Settings profile', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible({ timeout: 3000 }).catch(() => false)) await menu.click();
    await page.waitForTimeout(1000);
    await page.screenshot({ path: 'e2e-shots/s05-profile.png', fullPage: true });
  });

  test('06 Public key + QR', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible()) await menu.click();
    await page.waitForTimeout(800);
    const qr = page.locator('button:has-text("QR")');
    if (await qr.isVisible({ timeout: 1000 }).catch(() => false)) await qr.click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'e2e-shots/s06-qr.png', fullPage: true });
  });

  test('07 Advanced tab', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible()) await menu.click();
    await page.waitForTimeout(500);
    const gear = page.locator('button:has-text("⚙")').first();
    if (await gear.isVisible({ timeout: 2000 }).catch(() => false)) await gear.click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'e2e-shots/s07-advanced.png', fullPage: true });
  });

  test('08 E2E toggle click', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible()) await menu.click();
    await page.waitForTimeout(500);
    const gear = page.locator('button:has-text("⚙")').first();
    if (await gear.isVisible({ timeout: 2000 }).catch(() => false)) await gear.click();
    await page.waitForTimeout(500);
    const toggle = page.locator('button:has-text("ON"), button:has-text("OFF")').first();
    if (await toggle.isVisible({ timeout: 2000 }).catch(() => false)) {
      await toggle.click();
      await page.waitForTimeout(300);
    }
    await page.screenshot({ path: 'e2e-shots/s08-e2e-clicked.png', fullPage: true });
  });

  test('09 Alice demo', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const alice = page.locator('button:has-text("Alice"), button:has-text("T1")').first();
    if (await alice.isVisible({ timeout: 3000 }).catch(() => false)) {
      await alice.click();
      await page.waitForTimeout(2000);
    }
    await page.screenshot({ path: 'e2e-shots/s09-alice.png', fullPage: true });
  });

  test('10 Bob demo final', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(3000);
    const bob = page.locator('button:has-text("Bob"), button:has-text("T2")').first();
    if (await bob.isVisible({ timeout: 3000 }).catch(() => false)) {
      await bob.click();
      await page.waitForTimeout(2000);
    }
    await page.screenshot({ path: 'e2e-shots/s10-bob-final.png', fullPage: true });
  });
});
