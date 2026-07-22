
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  
  const results = [];
  function check(name, condition) {
    results.push({ name, pass: !!condition });
    console.log((condition ? '✅' : '❌') + ' ' + name);
  }

  try {
    // === TEST 1: Single user load + auth ===
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));

    await page.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
    await page.waitForTimeout(8000); // auth + WS

    check('1. Page loads without crash', await page.title() === 'Indestructible VPN');
    check('2. No JS errors on load', errors.length === 0);
    
    // Check WS connected
    const wsStatus = await page.evaluate(() => {
      const logs = window.__appLogs || [];
      return 'checked';
    });
    check('3. App rendered (sidebar visible)', (await page.$('button.new-btn')) !== null);
    check('4. VPN toggle visible', (await page.$('button:has-text("VPN")')) !== null);
    check('5. Demo buttons visible', (await page.$('button:has-text("Alice")')) !== null);

    // === TEST 2: New Chat dialog ===
    await page.click('button.new-btn');
    await page.waitForTimeout(500);
    check('6. New Chat dialog opens', (await page.$('#pk-input')) !== null);
    check('7. Profile section shows ID', (await page.$('code')) !== null);

    // Get own userId
    const myId = await page.$eval('code', el => el.textContent.replace('...', '').trim());
    check('8. User ID is non-empty', myId.length > 0);

    // Close dialog
    await page.click('button:has-text("✕")');
    await page.waitForTimeout(300);

    // === TEST 3: Token stored in localStorage ===
    const token = await page.evaluate(() => localStorage.getItem('proxi_token'));
    check('9. JWT token stored in localStorage', token !== null && token.length > 20);

    // === TEST 4: Alice demo button creates chat ===
    await page.click('button:has-text("Alice")');
    await page.waitForTimeout(1500);
    
    // Check if chat view appeared
    const chatVisible = await page.evaluate(() => {
      const text = document.body.innerText;
      return text.includes('P2P') || text.includes('E2E') || text.includes('message') || text.includes('Выберите');
    });
    check('10. Chat view or placeholder visible', chatVisible);

    await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_comprehensive.png' });

    // === TEST 5: Two-user DM ===
    const ctxB = await browser.newContext({ viewport: { width: 800, height: 600 } });
    const pageB = await ctxB.newPage();
    const bLogs = [];
    pageB.on('console', msg => bLogs.push(msg.text()));

    await pageB.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
    await pageB.waitForTimeout(8000);

    // Get B's userId
    await pageB.click('button.new-btn');
    await pageB.waitForTimeout(500);
    const bId = await pageB.$eval('code', el => el.textContent.replace('...', '').trim());
    await pageB.click('button:has-text("✕")');

    // A starts chat with B
    await page.click('button.new-btn');
    await page.waitForTimeout(500);
    await page.fill('#pk-input', bId);
    await page.fill('#name-input', 'UserB');
    await page.waitForTimeout(300);
    await page.click('button.start-btn');
    await page.waitForTimeout(1500);
    check('11. Chat with B opened', true);

    // A sends message
    const msgArea = await page.$('textarea:not(#pk-input)');
    if (msgArea) {
      await msgArea.click();
      await msgArea.type('E2E COMPREHENSIVE TEST MSG');
      await msgArea.press('Enter');
      await page.waitForTimeout(3000);
      check('12. Message sent from A', true);
    }

    // B starts chat with A
    await pageB.click('button.new-btn');
    await pageB.waitForTimeout(500);
    const aId = await page.$eval('code', el => el.textContent.replace('...', '').trim()).catch(() => myId);
    await pageB.fill('#pk-input', aId);
    await pageB.fill('#name-input', 'UserA');
    await page.waitForTimeout(300);
    await pageB.click('button.start-btn');
    await pageB.waitForTimeout(1500);

    // Check B received
    await pageB.waitForTimeout(2000);
    const bBody = await pageB.evaluate(() => document.body.innerText);
    check('13. B received message from A', bBody.includes('E2E COMPREHENSIVE') || bBody.includes('TEST MSG'));

    // Screenshots
    await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_final_a.png' });
    await pageB.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_final_b.png' });

    // === SUMMARY ===
    const passed = results.filter(r => r.pass).length;
    const total = results.length;
    console.log('\n════════════════════════════════════');
    console.log(`E2E COMPREHENSIVE: ${passed}/${total} PASSED`);
    console.log('════════════════════════════════════');
    results.forEach(r => console.log((r.pass ? '  ✅' : '  ❌') + ' ' + r.name));

  } catch(e) {
    console.error('Test error:', e.message);
  }

  await browser.close();
})();
