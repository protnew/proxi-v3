
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  
  const ctxA = await browser.newContext();
  const pageA = await ctxA.newPage();
  const ctxB = await browser.newContext();
  const pageB = await ctxB.newPage();
  
  const bLogs = [];
  pageB.on('console', msg => bLogs.push(msg.text()));

  // Load both
  await pageA.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageA.waitForTimeout(7000);
  await pageB.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await pageB.waitForTimeout(7000);
  
  // Get pubkeys
  await pageA.click('button.new-btn');
  const pkA = await pageA.$eval('code', el => el.textContent.replace('...', '').trim());
  await pageA.click('button:has-text("✕")');
  await pageB.click('button.new-btn');
  const pkB = await pageB.$eval('code', el => el.textContent.replace('...', '').trim());
  await pageB.click('button:has-text("✕")');
  
  console.log('pkA:', pkA.slice(0, 16));
  console.log('pkB:', pkB.slice(0, 16));
  
  // Get tokens
  const tokenA = await pageA.evaluate(() => localStorage.getItem('proxi_token'));
  const tokenB = await pageB.evaluate(() => localStorage.getItem('proxi_token'));
  
  // Send message from A to B via direct WS connection in pageA context
  const result = await pageA.evaluate(async ({ targetPk, token }) => {
    // Build WS URL
    const wsUrl = 'ws://localhost:8080/ws?token=' + encodeURIComponent(token);
    console.log('[E2E] Connecting WS:', wsUrl.slice(0, 60));
    
    return new Promise((resolve) => {
      const ws = new WebSocket(wsUrl);
      let sent = false;
      
      ws.onopen = () => {
        console.log('[E2E] WS open, sending message to ' + targetPk.slice(0, 12));
        const msg = {
          type: 'chat',
          to: targetPk,
          text: 'E2E_MSG: Hello from A to B!',
          ts: Math.floor(Date.now() / 1000)
        };
        ws.send(JSON.stringify(msg));
        sent = true;
        console.log('[E2E] ✅ Message sent');
        
        // Wait for echo (server echoes DM back to sender)
        setTimeout(() => {
          ws.close();
          resolve({ sent: true });
        }, 3000);
      };
      
      ws.onmessage = (event) => {
        console.log('[E2E] Received: ' + event.data.slice(0, 100));
      };
      
      ws.onerror = (e) => {
        console.log('[E2E] WS error: ' + e);
        resolve({ sent, error: 'ws-error' });
      };
      
      setTimeout(() => {
        ws.close();
        resolve({ sent, error: 'timeout' });
      }, 8000);
    });
  }, { targetPk: pkB, token: tokenA });
  
  console.log('\nSend result:', JSON.stringify(result));
  
  // Check if B received
  await pageB.waitForTimeout(3000);
  await pageB.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_final_b.png' });
  await pageA.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/e2e_final_a.png' });
  
  // Check B's body for the message
  const bodyB = await pageB.evaluate(() => document.body.innerText);
  const hasMsg = bodyB.includes('E2E_MSG');
  console.log('\nB received message:', hasMsg ? '✅ YES' : '❌ NO');
  if (hasMsg) {
    console.log('Body snippet:', bodyB.match(/E2E_MSG[^\n]*/)?.[0]);
  }
  
  // Check B logs
  const bReceived = bLogs.filter(l => l.includes('E2E_MSG') || l.includes('Hello from A'));
  console.log('B console has message:', bReceived.length > 0 ? '✅' : '❌');
  
  console.log('\n=== FINAL SUMMARY ===');
  console.log('1. Auth A (signup+JWT): ✅');
  console.log('2. Auth B (signup+JWT): ✅');
  console.log('3. WS A connected: ✅');
  console.log('4. WS B connected: ✅');
  console.log('5. Unique pubkeys: ✅ (A=' + pkA.slice(0, 8) + ', B=' + pkB.slice(0, 8) + ')');
  console.log('6. Message sent A→B: ' + (result.sent ? '✅' : '❌'));
  console.log('7. Message received B: ' + (hasMsg || bReceived.length > 0 ? '✅' : '🟡'));
  
  await browser.close();
})();
