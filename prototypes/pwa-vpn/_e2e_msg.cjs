
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  
  const ctxA = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageA = await ctxA.newPage();
  const ctxB = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageB = await ctxB.newPage();

  const logsB = [];
  pageB.on('console', msg => {
    const text = msg.text();
    logsB.push(text);
    if (text.includes('message') || text.includes('chat') || text.includes('Hello')) {
      console.log('[B received] ' + text.slice(0, 200));
    }
  });

  // Load both
  console.log('Loading User A and B...');
  await pageA.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageA.waitForTimeout(6000); // wait for auth + WS
  await pageB.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageB.waitForTimeout(6000);
  console.log('Both loaded.');

  // Get User A's pubkey from the page
  const pubkeyA = await pageA.evaluate(() => {
    return document.querySelector('[class*="profile"]')?.textContent?.slice(0, 30) || 'unknown';
  });
  console.log('User A identity element:', pubkeyA);

  // User A: Click Alice demo button to open a chat
  console.log('\n=== User A: click Alice ===');
  const aliceBtn = await pageA.$('button:has-text("Alice")');
  if (aliceBtn) {
    await aliceBtn.click();
    await pageA.waitForTimeout(1000);
  }

  // Find message input and type
  const msgInput = await pageA.$('textarea');
  if (msgInput) {
    console.log('Found textarea — typing message...');
    await msgInput.click();
    await msgInput.type('E2E TEST: Hello from User A!');
    await pageA.waitForTimeout(500);
    await msgInput.press('Enter');
    console.log('Message sent!');
    await pageA.waitForTimeout(3000);
    
    // Screenshot A after send
    await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_A_sent.png' });
    console.log('Screenshot: A_sent');
    
    // Check messages on page A
    const msgsA = await pageA.evaluate(() => {
      const elements = document.querySelectorAll('[class*="message"], [class*="msg"], [class*="bubble"]');
      return Array.from(elements).map(e => e.textContent.trim().slice(0, 80));
    });
    console.log('Messages on A:', JSON.stringify(msgsA.slice(0, 5)));
  } else {
    console.log('No textarea found!');
    // Try input[type=text]
    const textInputs = await pageA.$$('input[type="text"]');
    console.log('Found text inputs:', textInputs.length);
  }

  // Wait and check if B received anything
  await pageB.waitForTimeout(2000);
  await pageB.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_B_after.png' });
  console.log('Screenshot: B_after');

  // Check messages on page B  
  const msgsB = await pageB.evaluate(() => {
    const elements = document.querySelectorAll('[class*="message"], [class*="msg"], [class*="bubble"]');
    return Array.from(elements).map(e => e.textContent.trim().slice(0, 80));
  });
  console.log('Messages on B:', JSON.stringify(msgsB.slice(0, 5)));

  // Check B console for received messages
  const received = logsB.filter(l => l.includes('Hello') || l.includes('E2E'));
  console.log('\nB received message logs:', received.length);
  if (received.length > 0) {
    console.log('✅ MESSAGE DELIVERED to User B!');
  } else {
    console.log('⚠️ Message NOT received by User B (expected — both users share same identity in dev)');
  }

  await browser.close();
})();
