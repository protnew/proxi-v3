import { test, expect } from '@playwright/test';

test('QR Scanner UI renders and requests camera permissions', async ({ page }) => {
  await page.goto('http://localhost:4173/scan');
  
  // Fake the camera permission for testing
  await page.context().grantPermissions(['camera']);
  
  const videoElement = page.locator('video');
  await expect(videoElement).toBeVisible();
  
  const scanFrame = page.locator('.qr-scan-frame');
  await expect(scanFrame).toBeVisible();
});
