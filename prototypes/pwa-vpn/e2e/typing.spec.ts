import { test, expect, BrowserContext, Page } from '@playwright/test';
import { EventEmitter } from 'events';

test.describe('Typing Indicator E2E', () => {
  let contextA: BrowserContext;
  let contextB: BrowserContext;
  let pageA: Page;
  let pageB: Page;

  const bus = new EventEmitter();

  test.beforeAll(async ({ browser }) => {
    contextA = await browser.newContext({ viewport: { width: 1400, height: 900 } });
    contextB = await browser.newContext({ viewport: { width: 1400, height: 900 } });

    pageA = await contextA.newPage();
    pageB = await contextB.newPage();
  });

  test.afterAll(async () => {
    await contextA.close();
    await contextB.close();
  });

  test.beforeEach(async () => {
    const setupWS = async (page: Page) => {
      await page.routeWebSocket('**/ws*', ws => {
        const url = new URL(ws.url());
        const userId = url.searchParams.get('userId') || 'unknown';

        const handler = (msg: string) => {
          ws.send(msg);
        };
        bus.on('broadcast', handler);
        
        ws.onMessage(message => {
          try {
            const parsed = JSON.parse(message as string);
            parsed.from = userId;
            bus.emit('broadcast', JSON.stringify(parsed));
          } catch (e) {
            bus.emit('broadcast', message);
          }
        });
        
        ws.onClose(() => bus.off('broadcast', handler));
      });
    };

    await setupWS(pageA);
    await setupWS(pageB);
  });

  test('Alice typing shows "печатает..." for Bob', async () => {
    const appUrl = 'http://localhost:5175';
    
    await pageA.goto(appUrl);
    await pageB.goto(appUrl);

    // Дожидаемся загрузки
    await expect(pageA.locator('.sidebar')).toBeVisible({ timeout: 15000 });
    await expect(pageB.locator('.sidebar')).toBeVisible({ timeout: 15000 });

    const pubkeyA = await pageA.evaluate(() => localStorage.getItem('local-id'));
    const pubkeyB = await pageB.evaluate(() => localStorage.getItem('local-id'));
    expect(pubkeyA).toBeTruthy();
    expect(pubkeyB).toBeTruthy();

    // Alice opens chat with Bob
    await pageA.locator('.new-btn').click();
    await pageA.locator('#pk-input').fill(pubkeyB as string);
    await pageA.locator('.start-btn').click();
    const textareaA = pageA.locator('.input-area textarea');
    await expect(textareaA).toBeVisible();

    // Bob opens chat with Alice
    await pageB.locator('.new-btn').click();
    await pageB.locator('#pk-input').fill(pubkeyA as string);
    await pageB.locator('.start-btn').click();
    await expect(pageB.locator('.input-area textarea')).toBeVisible();

    // Bob checks initial status
    await expect(pageB.locator('.peer-status')).toHaveText(/был\(а\) недавно/, { timeout: 5000 });

    // Alice types: trigger handleKey (keydown Enter or input changes and typing anything)
    // fill does not necessarily trigger the manual keydown events or is too fast. 
    // We will type character by character
    await textareaA.pressSequentially('Hello', { delay: 100 });

    // Bob should see "печатает..."
    await expect(pageB.locator('.peer-status')).toHaveText(/печатает\.\.\./, { timeout: 5000 });
  });
});
