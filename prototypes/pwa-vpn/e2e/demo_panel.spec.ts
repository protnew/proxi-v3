import { test, expect } from '@playwright/test';

test.describe('Demo Panel E2E', () => {
  test('User can use Demo Panel to auto-connect and send message', async ({ browser }) => {
    test.setTimeout(60000); // 60 seconds
    const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });

    const pageA = await context.newPage();
    const pageB = await context.newPage();

    // 1. Открываем интерфейсы (используем один порт, так как разные контексты изолированы)
    await pageA.goto('http://localhost:5175');
    await pageB.goto('http://localhost:5175');

    // 2. Дожидаемся загрузки приложения (App.svelte скрывает лоадер)
    await pageA.waitForSelector('.demo-panel', { state: 'visible', timeout: 20000 });
    await pageB.waitForSelector('.demo-panel', { state: 'visible', timeout: 20000 });

    // 3. Логинимся через Demo Panel (с ожиданием перезагрузки страницы)
    await Promise.all([
      pageA.waitForNavigation(),
      pageA.locator('button:has-text("Alice (T1)")').click()
    ]);
    await Promise.all([
      pageB.waitForNavigation(),
      pageB.locator('button:has-text("Bob (T2)")').click()
    ]);

    // Проверяем статус
    await expect(pageA.locator('.demo-panel')).toContainText('Tester 1');
    await expect(pageB.locator('.demo-panel')).toContainText('Tester 2');

    // 4. Авто-коннект со стороны Алисы
    await pageA.locator('button:has-text("Auto-Connect")').click();

    // 5. Проверяем сообщения на стороне Алисы
    await expect(pageA.locator('.msg-text').first()).toContainText('Tester 1', { timeout: 15000 });

    // На стороне Боба чат появляется в сайдбаре, его нужно выбрать
    await pageB.locator('.chat-item').first().waitFor({ state: 'visible', timeout: 15000 });
    await pageB.locator('.chat-item').first().click();

    // Проверяем сообщение на стороне Боба
    await expect(pageB.locator('.msg-text').first()).toContainText('Tester 1', { timeout: 15000 });

    await context.close();
  });
});
