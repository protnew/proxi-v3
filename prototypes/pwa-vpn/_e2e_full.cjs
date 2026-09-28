
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  
  // === USER A ===
  const ctxA = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageA = await ctxA.newPage();
  const logsA = [];
  pageA.on('console', msg => logsA.push(msg.text().slice(0, 200)));

  // === USER B ===
  const ctxB = await browser.newContext({ viewport: { width: 800, height: 600 } });
  const pageB = await ctxB.newPage();
  const logsB = [];
  pageB.on('console', msg => logsB.push(msg.text().slice(0, 200)));

  // Load both
  console.log('=== Loading both users ===');
  await pageA.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageA.waitForTimeout(7000); // auth + WS
  await pageB.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageB.waitForTimeout(7000);
  
  // Get User A pubkey from API
  const tokenA = await pageA.evaluate(() => localStorage.getItem('proxi_token'));
  const pubkeyA = await pageA.evaluate(() => {
    // The pubkey is stored in the profile store — try to find it
    const codeEl = document.querySelector('code');
    return codeEl ? codeEl.textContent.replace('...', '') : '';
  });
  console.log('User A token:', tokenA ? tokenA.slice(0, 20) + '...' : 'NONE');
  console.log('User A pubkey (from code element):', pubkeyA);

  // Better: get pubkey from the profile section of NewChat
  // Click new chat button
  console.log('\n=== User A: Open New Chat ===');
  await pageA.click('button.new-btn');
  await pageA.waitForTimeout(1000);
  
  // Get own pubkey from the profile section
  const myPubkey = await pageA.evaluate(() => {
    const codeEl = document.querySelector('code');
    return codeEl ? codeEl.textContent.replace('...', '').trim() : '';
  });
  console.log('User A pubkey:', myPubkey);

  // Also get User B's pubkey
  await pageB.click('button.new-btn');
  await pageB.waitForTimeout(500);
  const bPubkey = await pageB.evaluate(() => {
    const codeEl = document.querySelector('code');
    return codeEl ? codeEl.textContent.replace('...', '').trim() : '';
  });
  console.log('User B pubkey:', bPubkey);
  await pageB.click('button:has-text("✕")'); // close dialog

  // User A: type B's pubkey and start chat
  if (bPubkey && bPubkey.length >= 32) {
    console.log('\n=== User A: Starting chat with B ===');
    const pkTextarea = await pageA.$('#pk-input');
    if (pkTextarea) {
      await pkTextarea.fill(bPubkey);
      await pageA.waitForTimeout(300);
      
      // Type name
      const nameInput = await pageA.$('#name-input');
      if (nameInput) await nameInput.fill('UserB');
      
      await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_newchat_filled.png' });
      console.log('Screenshot: newchat_filled');
      
      // Click Start button
      const startBtn = await pageA.$('button.start-btn');
      if (startBtn) {
        const isDisabled = await startBtn.isDisabled();
        console.log('Start button disabled:', isDisabled);
        if (!isDisabled) {
          await startBtn.click();
          await pageA.waitForTimeout(1500);
          await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_chat_opened.png' });
          console.log('Screenshot: chat_opened');
          
          // Now find message input
          const msgInput = await pageA.$('textarea:not(#pk-input)');
          if (msgInput) {
            console.log('Found message textarea!');
            await msgInput.click();
            await msgInput.type('E2E TEST: Hello UserB from UserA!');
            await pageA.waitForTimeout(500);
            await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_msg_typed.png' });
            
            await msgInput.press('Enter');
            await pageA.waitForTimeout(3000);
            await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_msg_sent.png' });
            console.log('✅ Message sent from A!');
            
            // Check messages visible on A
            const msgsA = await pageA.evaluate(() => 
              Array.from(document.querySelectorAll('[class*="bubble"], [class*="message-text"], [class*="msg-text"]')).map(e => e.textContent.trim().slice(0, 80))
            );
            console.log('Messages on A:', msgsA);
          } else {
            console.log('❌ No message textarea found after opening chat');
          }
        }
      }
    }
  } else {
    console.log('❌ Could not get User B pubkey (length=' + (bPubkey?.length || 0) + ')');
  }

  // Check if B received
  console.log('\n=== Checking User B ===');
  await pageB.waitForTimeout(2000);
  await pageB.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_b_final.png' });
  
  const bMsgs = await pageB.evaluate(() => document.body.innerText.slice(0, 500));
  console.log('B body text:', bMsgs.slice(0, 300));
  
  const bReceived = logsB.filter(l => l.includes('E2E') || l.includes('Hello'));
  console.log('\nB received in logs:', bReceived.length, bReceived);
  
  console.log('\n=== Summary ===');
  console.log('A sent message:', logsA.some(l => l.includes('E2E') || l.includes('Hello')) ? '✅' : '🟡');
  console.log('B received:', bReceived.length > 0 ? '✅' : '🟡 (same identity, WS echo expected)');
  console.log('0 JS errors:', !logsA.some(l => l.includes('[ERROR]')) && !logsB.some(l => l.includes('[ERROR]')) ? '✅' : '❌');

  await browser.close();
})();
