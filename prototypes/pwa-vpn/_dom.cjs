
const { chromium } = require('playwright');

(async () => {
  const browserPath = 'C:/Users/Space/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe';
  const browser = await chromium.launch({ headless: true, executablePath: browserPath });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  
  await page.goto('http://localhost:5173', { waitUntil: 'networkidle', timeout: 15000 });
  await page.waitForTimeout(6000);
  
  // Screenshot before clicking
  await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/dom_before.png' });
  console.log('Before click screenshot taken');
  
  // Get ALL interactive elements
  const allElements = await page.evaluate(() => {
    const items = [];
    document.querySelectorAll('button, a, input, textarea, [contenteditable], [role="button"]').forEach(el => {
      items.push({
        tag: el.tagName,
        type: el.type || '',
        text: el.textContent?.trim().slice(0, 50) || '',
        placeholder: el.placeholder || '',
        class: el.className?.slice(0, 60) || '',
        visible: el.offsetParent !== null
      });
    });
    return items;
  });
  console.log('\n=== All interactive elements ===');
  allElements.forEach((el, i) => {
    if (el.visible) console.log(`${i}: <${el.tag} type="${el.type}" class="${el.class}"> "${el.text}" placeholder="${el.placeholder}"`);
  });
  
  // Click Alice
  const alice = await page.$('button:has-text("Alice")');
  if (alice) {
    await alice.click();
    await page.waitForTimeout(2000);
    await page.screenshot({ path: 'C:/Users/Space/AppData/Local/Temp/dom_after_alice.png' });
    console.log('\n=== After Alice click ===');
    
    // Check what's visible now
    const afterElements = await page.evaluate(() => {
      const items = [];
      document.querySelectorAll('input, textarea, [contenteditable], button[type="submit"], form').forEach(el => {
        items.push({
          tag: el.tagName,
          type: el.type || '',
          placeholder: el.placeholder || '',
          class: el.className?.slice(0, 60) || '',
          visible: el.offsetParent !== null
        });
      });
      return items;
    });
    afterElements.forEach(el => {
      if (el.visible) console.log(`<${el.tag} type="${el.type}" class="${el.class}"> placeholder="${el.placeholder}"`);
    });
    
    // Get full body text to see chat
    const bodyText = await page.evaluate(() => document.body.innerText.slice(0, 1000));
    console.log('\nBody text:', bodyText);
  }
  
  await browser.close();
})();
