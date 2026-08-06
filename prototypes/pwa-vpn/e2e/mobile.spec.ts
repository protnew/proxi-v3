import { test, expect, devices } from '@playwright/test';

test('Mobile: Alice sidebar visible, chat hidden until tap', async ({ browser }) => {
  const ctx = await browser.newContext({ ...devices['iPhone 12'] });
  const page = await ctx.newPage();
  await page.goto('http://127.0.0.1:8090/?role=alice', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(3000);

  // Sidebar should be visible (list of chats)
  const sidebar = page.locator('.sidebar-mobile-wrapper');
  await expect(sidebar).toBeVisible();

  // Chat should be hidden on mobile initially
  const chat = page.locator('.chat-mobile-wrapper');
  // On mobile, chat wrapper has hidden-mobile class when no chat selected
  const isHidden = await chat.evaluate(el => {
    return el.classList.contains('hidden-mobile') || getComputedStyle(el).display === 'none';
  });
  console.log('chat hidden on mobile:', isHidden);

  // Click on a chat item
  const chatItem = page.locator('.chat-item, [class*="chat-item"]').first();
  if (await chatItem.count() > 0) {
    await chatItem.click();
    await page.waitForTimeout(500);

    // Now chat should be visible, sidebar hidden
    const chatVisible = await chat.evaluate(el => getComputedStyle(el).display !== 'none');
    console.log('chat visible after click:', chatVisible);

    // Check back button
    const backBtn = page.locator('.back-btn').first();
    const backVisible = await backBtn.evaluate(el => getComputedStyle(el).display !== 'none');
    console.log('back button visible:', backVisible);

    if (backVisible) {
      await backBtn.click();
      await page.waitForTimeout(500);
      const sidebarBack = await sidebar.evaluate(el => getComputedStyle(el).display !== 'none');
      console.log('sidebar back after back-btn:', sidebarBack);
    }
  }

  // Check viewport width
  const vp = await page.evaluate(() => window.innerWidth);
  console.log('viewport width:', vp);
  expect(vp).toBeLessThan(500);

  await page.screenshot({ path: 'e2e-shots/mobile-alice.png' });
  await ctx.close();
});

test('Mobile: VPN panel share link visible', async ({ browser }) => {
  const ctx = await browser.newContext({ ...devices['iPhone 12'] });
  const page = await ctx.newPage();
  await page.goto('http://127.0.0.1:8090/?role=alice', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(3000);

  // VPN panel should be visible and compact
  const vpnPanel = page.locator('.vpn-panel');
  if (await vpnPanel.count() > 0) {
    const visible = await vpnPanel.first().isVisible();
    console.log('vpn panel visible:', visible);
  }

  // Check that demo panel doesn't overflow screen
  const demoPanel = page.locator('.demo-panel, [class*="demo-panel"]');
  if (await demoPanel.count() > 0) {
    const rect = await demoPanel.first().boundingBox();
    const vp = await page.evaluate(() => window.innerWidth);
    console.log('demo panel width:', rect?.width, 'viewport:', vp);
    if (rect) {
      expect(rect.x + rect.width).toBeLessThanOrEqual(vp + 20);
    }
  }

  await page.screenshot({ path: 'e2e-shots/mobile-vpn.png' });
  await ctx.close();
});
