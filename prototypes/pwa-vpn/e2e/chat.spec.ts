import { test, expect, BrowserContext, Page } from '@playwright/test';
import * as http from 'http';

test.describe('Chat E2E: Отправка сообщения между 2 профилями', () => {
  let contextA: BrowserContext;
  let contextB: BrowserContext;
  let pageA: Page;
  let pageB: Page;
  let mockServer: http.Server;

  // Simple SSE and POST mock server
  const clients = new Set<http.ServerResponse>();

  test.beforeAll(async ({ browser }) => {
    mockServer = http.createServer((req, res) => {
      // CORS headers
      res.setHeader('Access-Control-Allow-Origin', '*');
      res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
      res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
      if (req.method === 'OPTIONS') { res.writeHead(200); return res.end(); }

      if (req.method === 'POST' && req.url === '/api/dm') {
        let body = '';
        req.on('data', chunk => body += chunk);
        req.on('end', () => {
          clients.forEach(c => c.write(`data: ${body}\n\n`));
          res.writeHead(200); res.end('{"status":"ok"}');
        });
      } else if (req.method === 'GET' && req.url === '/api/stream') {
        res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Connection': 'keep-alive' });
        clients.add(res);
        req.on('close', () => clients.delete(res));
      } else {
        res.writeHead(404); res.end();
      }
    });
    mockServer.listen(8080);

    // Инициализируем два полностью изолированных контекста браузера (Desktop)
    contextA = await browser.newContext({ viewport: { width: 1400, height: 900 } });
    contextB = await browser.newContext({ viewport: { width: 1400, height: 900 } });

    pageA = await contextA.newPage();
    pageB = await contextB.newPage();
  });

  test.afterAll(async () => {
    await contextA.close();
    await contextB.close();
    mockServer.close();
  });

  test.afterAll(async () => {
    await contextA.close();
    await contextB.close();
  });

  test('Пользователь А отправляет сообщение Пользователю Б', async () => {
    const appUrl = 'http://localhost:5175';
    
    // Слушаем консоль страницы для дебага
    pageA.on('console', msg => console.log(`[Page A] ${msg.type()}: ${msg.text()}`));
    pageA.on('pageerror', error => console.log(`[Page A ERROR]: ${error.message}`));
    pageB.on('console', msg => console.log(`[Page B] ${msg.type()}: ${msg.text()}`));
    pageB.on('pageerror', error => console.log(`[Page B ERROR]: ${error.message}`));

    // 1. Открываем приложение
    await pageA.goto(appUrl);
    await pageB.goto(appUrl);

    // Дожидаемся инициализации (скрытие лоадера)
    try {
      await expect(pageA.locator('.sidebar')).toBeVisible({ timeout: 15000 });
      await expect(pageB.locator('.sidebar')).toBeVisible({ timeout: 15000 });
    } catch (e) {
      await pageA.screenshot({ path: 'test-results/pageA-fail.png' });
      await pageB.screenshot({ path: 'test-results/pageB-fail.png' });
      console.log('Page A HTML:', await pageA.content());
      throw e;
    }

    // 2. Получаем pubkey Пользователя Б
    const pubkeyB = await pageB.evaluate(() => localStorage.getItem('local-id'));
    expect(pubkeyB).toBeTruthy();

    // 3. Пользователь А создает новый чат с Пользователем Б
    await pageA.locator('.new-btn').click();
    await pageA.locator('#pk-input').fill(pubkeyB as string);
    await pageA.locator('.start-btn').click();

    // 4. Ожидаем открытия чата у Пользователя А (появление textarea)
    const textareaA = pageA.locator('.input-area textarea');
    await expect(textareaA).toBeVisible();

    // Пользователь Б тоже должен открыть чат с А, чтобы получать сообщения напрямую (или увидеть в сайдбаре)
    const pubkeyA = await pageA.evaluate(() => localStorage.getItem('local-id'));
    await pageB.locator('.new-btn').click();
    await pageB.locator('#pk-input').fill(pubkeyA as string);
    await pageB.locator('.start-btn').click();
    
    await expect(pageB.locator('.input-area textarea')).toBeVisible();

    // 5. Пользователь А отправляет сообщение
    const timestamp = Date.now();
    const uniqueMessageText = `Hello from E2E Test - ${timestamp}`;
    
    await textareaA.fill(uniqueMessageText);
    await pageA.locator('.send-btn').click();

    // 6. Проверяем, что сообщение появилось у Пользователя А
    const bubbleA = pageA.locator('.bubble', { hasText: uniqueMessageText });
    await expect(bubbleA).toBeVisible({ timeout: 5000 });

    // 7. Проверяем, что сообщение появилось у Пользователя Б
    const bubbleB = pageB.locator('.bubble', { hasText: uniqueMessageText });
    await expect(bubbleB).toBeVisible({ timeout: 10000 });
  });
});
