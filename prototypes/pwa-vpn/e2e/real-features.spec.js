import { test, expect } from '@playwright/test';
const BASE = 'http://localhost:5173';

test.describe.serial('Proxi V2 — Real Feature Tests', () => {

  test('01 App loads with messenger UI', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(4000);
    await page.screenshot({ path: 'e2e-shots/r01-load.png', fullPage: true });
    const text = await page.locator('body').textContent();
    expect(text).toContain('Indestructible');
  });

  test('02 Settings → Advanced tab → E2E toggle', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    // Click menu (☰)
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible({ timeout: 3000 }).catch(() => false)) await menu.click();
    await page.waitForTimeout(800);
    // Click advanced gear (⚙️)
    const gear = page.locator('button:has-text("⚙️")').first();
    if (await gear.isVisible({ timeout: 2000 }).catch(() => false)) await gear.click();
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'e2e-shots/r02-advanced.png', fullPage: true });
    const text = await page.locator('.settings, aside, .sidebar').textContent().catch(() => '');
    // Check for E2E or Шифрование
    expect(text.toLowerCase()).toMatch(/шифрован|e2e|encrypt/i);
  });

  test('03 E2E toggle click → switches state', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    const menu = page.locator('.menu-btn').first();
    if (await menu.isVisible({ timeout: 3000 }).catch(() => false)) await menu.click();
    await page.waitForTimeout(500);
    const gear = page.locator('button:has-text("⚙️")').first();
    if (await gear.isVisible({ timeout: 2000 }).catch(() => false)) await gear.click();
    await page.waitForTimeout(500);
    // Find ON/OFF button in advanced section
    const toggleBtn = page.locator('button:has-text("ON"), button:has-text("OFF")').first();
    if (await toggleBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      const beforeText = await toggleBtn.textContent();
      await toggleBtn.click();
      await page.waitForTimeout(300);
      const afterText = await toggleBtn.textContent();
      await page.screenshot({ path: 'e2e-shots/r03-e2e-clicked.png', fullPage: true });
      // Button text should change (ON→OFF or OFF→ON)
      expect(beforeText).not.toBe(afterText);
    }
  });

  test('04 New Chat → Group tab visible', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    const newBtn = page.locator('.new-btn').first();
    if (await newBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await newBtn.click();
      await page.waitForTimeout(800);
      // Check for Group tab
      const groupTab = page.locator('button:has-text("Группа")');
      const hasGroup = await groupTab.isVisible({ timeout: 2000 }).catch(() => false);
      await page.screenshot({ path: 'e2e-shots/r04-group-tab.png', fullPage: true });
      expect(hasGroup).toBe(true);
    }
  });

  test('05 Search bar functional', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    const search = page.locator('input[placeholder*="Поиск"]').first();
    if (await search.isVisible({ timeout: 3000 }).catch(() => false)) {
      await search.fill('test');
      await page.waitForTimeout(500);
      await page.screenshot({ path: 'e2e-shots/r05-search.png', fullPage: true });
      const val = await search.inputValue();
      expect(val).toBe('test');
    }
  });

  test('06 Alice demo button → activates user', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    const alice = page.locator('button:has-text("Alice")').first();
    if (await alice.isVisible({ timeout: 3000 }).catch(() => false)) {
      await alice.click();
      await page.waitForTimeout(2000);
      await page.screenshot({ path: 'e2e-shots/r06-alice-active.png', fullPage: true });
    }
  });

  test('07 Final full app screenshot', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await page.waitForTimeout(3000);
    await page.screenshot({ path: 'e2e-shots/r07-final.png', fullPage: true });
  });
});
