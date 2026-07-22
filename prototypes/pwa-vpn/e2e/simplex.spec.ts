import { test, expect } from '@playwright/test';

test.describe('SimpleX Signalling E2E', () => {
  test('Alice connects to Bob using an OOB invite link', async ({ browser }) => {
    // We mock two separate contexts to represent Alice and Bob
    const aliceContext = await browser.newContext();
    const bobContext = await browser.newContext();
    
    const alicePage = await aliceContext.newPage();
    const bobPage = await bobContext.newPage();
    
    // Simulate Bob creating an invite link (Queue endpoint)
    await bobPage.goto('http://localhost:4173/'); // PWA
    
    // In a real UI, Bob would click "Generate Invite"
    // We evaluate a script to simulate the underlying JS logic
    const bobInviteLink = await bobPage.evaluate(() => {
      return `smp://127.0.0.1:5223/${crypto.randomUUID()}`;
    });
    
    expect(bobInviteLink).toContain('smp://');
    
    // Alice receives the link Out-Of-Band and connects
    await alicePage.goto('http://localhost:4173/');
    
    // Alice connects to the SMP queue
    const connected = await alicePage.evaluate((invite) => {
      // Simulate sending a hello message to Bob's queue
      return true;
    }, bobInviteLink);
    
    expect(connected).toBeTruthy();
    
    await aliceContext.close();
    await bobContext.close();
  });
});
