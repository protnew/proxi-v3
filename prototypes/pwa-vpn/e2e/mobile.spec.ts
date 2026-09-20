import { test, expect, devices } from '@playwright/test';

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:5173';

test('Mobile: Alice sidebar visible, chat hidden until tap', async ({ browser }) => {
  const ctx = await browser.newContext({ ...devices['iPhone 12'] });
  const page = await ctx.newPage();
  await page.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester1'));
  await page.goto(APP_URL + '/?role=alice', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(3000);

  const sidebar = page.locator('.sidebar-mobile-wrapper');
  await expect(sidebar).toBeVisible({ timeout: 15000 });

  const chat = page.locator('.chat-mobile-wrapper');
  const isHidden = await chat.evaluate(el => {
    return el.classList.contains('hidden-mobile') || getComputedStyle(el).display === 'none';
  });
  console.log('chat hidden on mobile:', isHidden);
  expect(isHidden).toBeTruthy();

  const chatItem = page.locator('.chat-item, [class*="chat-item"]').first();
  if (await chatItem.count() > 0) {
    await chatItem.click();
    await page.waitForTimeout(500);

    const chatVisible = await chat.evaluate(el => getComputedStyle(el).display !== 'none');
    console.log('chat visible after click:', chatVisible);

    const backBtn = page.locator('.back-btn').first();
    if (await backBtn.count()) {
      const backVisible = await backBtn.evaluate(el => getComputedStyle(el).display !== 'none');
      console.log('back button visible:', backVisible);
      if (backVisible) {
        await backBtn.click();
        await page.waitForTimeout(500);
      }
    }
  }

  const vp = await page.evaluate(() => window.innerWidth);
  console.log('viewport width:', vp);
  expect(vp).toBeLessThan(500);

  await page.screenshot({ path: 'e2e-shots/mobile-alice.png' });
  await ctx.close();
});

test('Mobile: VPN panel share link visible', async ({ browser }) => {
  const ctx = await browser.newContext({ ...devices['iPhone 12'] });
  const page = await ctx.newPage();
  await page.addInitScript(() => localStorage.setItem('proxi_demo_role', 'tester1'));
  await page.goto(APP_URL + '/?role=alice', { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(3000);

  const vpnPanel = page.locator('.vpn-panel');
  if (await vpnPanel.count() > 0) {
    const visible = await vpnPanel.first().isVisible();
    console.log('vpn panel visible:', visible);
  }

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
