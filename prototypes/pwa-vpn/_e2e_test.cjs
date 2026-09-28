
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  
  // === USER A ===
  const ctxA = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageA = await ctxA.newPage();
  const logsA = [];
  pageA.on('console', msg => logsA.push('[A:' + msg.type() + '] ' + msg.text().slice(0, 200)));
  pageA.on('pageerror', err => logsA.push('[A:ERROR] ' + err.message.slice(0, 200)));
  
  // === USER B ===
  const ctxB = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageB = await ctxB.newPage();
  const logsB = [];
  pageB.on('console', msg => logsB.push('[B:' + msg.type() + '] ' + msg.text().slice(0, 200)));
  pageB.on('pageerror', err => logsB.push('[B:ERROR] ' + err.message.slice(0, 200)));

  console.log('=== Loading app for User A ===');
  await pageA.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageA.waitForTimeout(5000); // Wait for initIdentityAsync + signup + WS connect

  console.log('=== Loading app for User B ===');
  await pageB.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageB.waitForTimeout(5000);

  // Screenshot both
  await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_userA_loaded.png' });
  await pageB.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_userB_loaded.png' });
  console.log('Screenshots: userA_loaded, userB_loaded');

  // Check WS status for both
  const statusA = await pageA.evaluate(() => {
    // Try to read the status from the app
    const wsInfo = window.__wsStatus || 'no ws status variable';
    return wsInfo;
  });
  
  // Check console logs for WS connection
  const wsA = logsA.filter(l => l.includes('Nostr status') || l.includes('connected'));
  const wsB = logsB.filter(l => l.includes('Nostr status') || l.includes('connected'));
  console.log('\nWS User A:', wsA.join('; '));
  console.log('WS User B:', wsB.join('; '));

  // Check for errors
  const errorsA = logsA.filter(l => l.includes('ERROR'));
  const errorsB = logsB.filter(l => l.includes('ERROR'));
  console.log('\nErrors A:', errorsA.length, errorsA.slice(0, 3).join('; '));
  console.log('Errors B:', errorsB.length, errorsB.slice(0, 3).join('; '));

  // === TRY SEND MESSAGE ===
  // User A: click "New Chat" (pencil button) or Alice demo button
  console.log('\n=== User A: trying to start a chat ===');
  
  // Get all buttons
  const buttonsA = await pageA.evaluate(() => 
    Array.from(document.querySelectorAll('button')).map(b => ({
      text: b.textContent.trim().slice(0, 40),
      classes: b.className.slice(0, 50)
    }))
  );
  console.log('Buttons on A:', JSON.stringify(buttonsA.slice(0, 8)));

  // Click pencil/new chat button
  const newChatBtn = await pageA.$('button:has-text("✏")');
  if (newChatBtn) {
    await newChatBtn.click();
    await pageA.waitForTimeout(1000);
    await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_newchat.png' });
    console.log('Screenshot: newchat');
    
    // Check what appeared
    const inputs = await pageA.$$('input, textarea');
    console.log('Inputs after new chat:', inputs.length);
    if (inputs.length > 0) {
      // Type a pubkey
      await inputs[0].fill('test-recipient-pubkey');
      await pageA.waitForTimeout(500);
      await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_typed.png' });
      console.log('Screenshot: typed recipient');
    }
  }

  // Try clicking Alice demo button  
  const aliceBtn = await pageA.$('button:has-text("Alice")');
  if (aliceBtn) {
    await aliceBtn.click();
    await pageA.waitForTimeout(1000);
    await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_alice_chat.png' });
    console.log('Screenshot: alice chat opened');
    
    // Find message input
    const msgInput = await pageA.$('textarea, input[type="text"]:not([style*="display:none"])');
    if (msgInput) {
      await msgInput.click();
      await msgInput.type('Hello from User A! This is a real E2E test.');
      await pageA.waitForTimeout(500);
      await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_message_typed.png' });
      console.log('Screenshot: message typed');
      
      await msgInput.press('Enter');
      await pageA.waitForTimeout(2000);
      await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_message_sent.png' });
      console.log('Screenshot: message sent');
    }
  }

  // Final: dump all console logs
  console.log('\n=== ALL LOGS USER A ===');
  logsA.forEach(l => console.log(l));
  console.log('\n=== ALL LOGS USER B ===');
  logsB.forEach(l => console.log(l));

  await browser.close();
})();
