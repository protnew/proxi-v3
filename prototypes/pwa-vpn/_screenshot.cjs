const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch({
    headless: true,
    executablePath: 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe'
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });

  const logs = [];
  page.on('console', msg => logs.push('[' + msg.type() + '] ' + msg.text()));
  page.on('pageerror', err => logs.push('[ERROR] ' + err.message));

  try {
    await page.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  } catch(e) {
    console.log('Navigation error: ' + e.message);
  }
  await page.waitForTimeout(3000);

  await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_main.png' });
  console.log('Screenshot 1: main screen');

  // Click Alice
  try {
    const alice = await page.$('text=Alice');
    if (alice) {
      await alice.click();
      await page.waitForTimeout(1500);
      await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_alice.png' });
      console.log('Screenshot 2: Alice chat opened');

      // Type message
      const input = await page.$('textarea, input[type="text"], [contenteditable="true"]');
      if (input) {
        await input.click();
        await input.type('Hello from Proxi smoke test!');
        await page.waitForTimeout(500);
        await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_typing.png' });
        console.log('Screenshot 3: typing message');
        await input.press('Enter');
        await page.waitForTimeout(2000);
        await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_sent.png' });
        console.log('Screenshot 4: message sent');
      } else {
        console.log('No input field found — taking screenshot of current state');
        await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_alice.png' });
      }
    } else {
      console.log('Alice button not found');
    }
  } catch(e) {
    console.log('Alice click error: ' + e.message);
  }

  // VPN panel
  try {
    const vpn = await page.$('text=VPN');
    if (vpn) {
      await vpn.click();
      await page.waitForTimeout(1000);
      await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/proxi_vpn.png' });
      console.log('Screenshot 5: VPN panel');
    }
  } catch(e) {}

  console.log('\n--- Console logs ---');
  logs.forEach(l => console.log(l));

  await browser.close();
})();
