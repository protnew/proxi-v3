const puppeteer = require('puppeteer');

(async () => {
  const browser = await puppeteer.launch();
  const page = await browser.newPage();
  
  page.on('console', msg => console.log('PAGE LOG:', msg.text()));
  page.on('pageerror', error => console.log('PAGE ERROR:', error.message));
  page.on('response', response => {
    if (!response.ok()) console.log('HTTP ERROR:', response.status(), response.url());
  });
  page.on('requestfailed', request => {
    console.log('REQUEST FAILED:', request.failure().errorText, request.url());
  });

  try {
    await page.goto('http://localhost:9999', { waitUntil: 'networkidle0', timeout: 10000 });
  } catch (e) {
    console.log('GOTO ERROR:', e.message);
  }

  await browser.close();
})();
