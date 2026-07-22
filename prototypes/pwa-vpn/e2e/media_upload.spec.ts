import { test, expect, BrowserContext, Page } from '@playwright/test';
import * as http from 'http';
import * as fs from 'fs';
import * as path from 'path';

test.describe('Media Upload E2E: Загрузка бинарного файла', () => {
  let contextA: BrowserContext;
  let contextB: BrowserContext;
  let pageA: Page;
  let pageB: Page;
  let mockServer: http.Server;
  const testFileName = 'test_binary_voice.webm';
  const testFilePath = path.join(process.cwd(), 'e2e', testFileName);

  const clients = new Set<http.ServerResponse>();

  test.beforeAll(async ({ browser }) => {
    // Создаем тестовый бинарный файл (голос)
    fs.writeFileSync(testFilePath, Buffer.from('dummy audio content binary data'));

    mockServer = http.createServer((req, res) => {
      // CORS
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
    if (fs.existsSync(testFilePath)) {
      fs.unlinkSync(testFilePath);
    }
  });

  test('Пользователь А загружает бинарный файл, и он отображается у обоих', async () => {
    const appUrl = 'http://localhost:5175';
    
    // Для дебага
    pageA.on('console', msg => console.log(`[Page A] ${msg.type()}: ${msg.text()}`));
    pageA.on('pageerror', error => console.log(`[Page A ERROR]: ${error.message}`));
    pageB.on('console', msg => console.log(`[Page B] ${msg.type()}: ${msg.text()}`));
    pageB.on('pageerror', error => console.log(`[Page B ERROR]: ${error.message}`));

    // 1. Открываем приложение
    await pageA.goto(appUrl);
    await pageB.goto(appUrl);

    try {
      await expect(pageA.locator('.sidebar')).toBeVisible({ timeout: 15000 });
      await expect(pageB.locator('.sidebar')).toBeVisible({ timeout: 15000 });
    } catch (e) {
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

    // 4. Ожидаем открытия чата у Пользователя А
    await expect(pageA.locator('.input-area textarea')).toBeVisible();

    // Пользователь Б тоже открывает чат
    const pubkeyA = await pageA.evaluate(() => localStorage.getItem('local-id'));
    await pageB.locator('.new-btn').click();
    await pageB.locator('#pk-input').fill(pubkeyA as string);
    await pageB.locator('.start-btn').click();
    
    await expect(pageB.locator('.input-area textarea')).toBeVisible();

    // 5. Пользователь А отправляет (загружает) бинарный файл через input
    await pageA.setInputFiles('#f-in', testFilePath);

    // 6. Проверяем, что файл отображается у пользователя А (как '.file-msg')
    const fileMsgA = pageA.locator('.file-msg', { hasText: testFileName });
    await expect(fileMsgA).toBeVisible({ timeout: 5000 });
    await expect(fileMsgA.locator('.fsize')).toBeVisible(); // Проверка, что размер тоже рендерится

    // 7. Проверяем, что файл появился у пользователя Б
    const fileMsgB = pageB.locator('.file-msg', { hasText: testFileName });
    await expect(fileMsgB).toBeVisible({ timeout: 10000 });
    await expect(fileMsgB.locator('.fsize')).toBeVisible();
  });
});
