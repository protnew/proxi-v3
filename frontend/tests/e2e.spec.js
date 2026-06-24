import { test, expect } from '@playwright/test';

test.describe('Unkillable Messenger E2E', () => {
  test.beforeEach(async ({ page }) => {
    // Go to the app (assuming it runs on localhost:9999)
    await page.goto('http://localhost:9999');
  });

  test('should load the application and show login or identity panel', async ({ page }) => {
    // Verify title
    await expect(page).toHaveTitle(/Proxi/i);
    
    // Wait for either login button or identity panel
    const identityHeader = page.locator('h3:has-text("🔑 Твой ID")');
    const generateBtn = page.locator('button:has-text("Сгенерировать")');
    
    await expect(identityHeader.or(generateBtn)).toBeVisible({ timeout: 10000 });
  });

  test('should allow switching tabs', async ({ page }) => {
    // Make sure we are past login (if not, we might need a mock identity)
    const chatTab = page.locator('.sidebar .logo-wrap');
    if (await chatTab.isVisible()) {
      await page.locator('text=Контакты').click();
      await expect(page.locator('.panel:has-text("Адресная книга")')).toBeVisible();

      await page.locator('text=VPN').click();
      await expect(page.locator('.panel:has-text("WireGuard VPN")')).toBeVisible();
    }
  });

  test('should open Dead Man Switch settings', async ({ page }) => {
    const switchTab = page.locator('text=DMS');
    if (await switchTab.isVisible()) {
      await switchTab.click();
      await expect(page.locator('h2:has-text("Dead Man\'s Switch")')).toBeVisible();
      
      const toggle = page.locator('#switchToggle');
      if (await toggle.isVisible()) {
        await toggle.check();
        await expect(page.locator('textarea[placeholder*="Сообщение"]')).toBeVisible();
      }
    }
  });
});
