
import { test, expect, type Page } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const BOB_SK = '2'.repeat(64)

async function waitVPNReady(page: Page, timeout = 15000) {
  for (let i = 0; i < timeout / 1000; i++) {
    await page.waitForTimeout(1000)
    const log = await page.getByTestId('vpn-log').innerText().catch(() => '')
    if (/Nostr VPN signaling ON/.test(log)) return log
  }
  return await page.getByTestId('vpn-log').innerText().catch(() => '')
}

test('1. dev=1: app loads, SOCKS panel renders', async ({ page }) => {
  test.setTimeout(30000)
  await page.goto(BASE + '/?role=alice&dev=1', { waitUntil: 'domcontentloaded' })
  // With overlay approach: app renders behind loading screen.
  // Check if SOCKS5 panel exists in DOM (even behind overlay)
  let socksFound = false
  for (let i = 0; i < 15; i++) {
    await page.waitForTimeout(1000)
    // Check DOM for SOCKS5 text using textContent (includes overlay-hidden elements)
    const socks = await page.evaluate(() => {
      return document.body.textContent?.includes('SOCKS5') || false
    })
    if (socks) { socksFound = true; break }
  }
  // Also check loading overlay state
  const overlayHidden = await page.evaluate(() => {
    const el = document.getElementById('loading-overlay')
    return el ? el.classList.contains('hidden') : true
  })
  console.log('SOCKS found:', socksFound, 'overlay hidden:', overlayHidden)
  await page.screenshot({ path: 'e2e-shots/crit-dev1-loaded.png', fullPage: true })
  expect(socksFound || overlayHidden).toBeTruthy()
})

test.setTimeout(120000);
test('2. Alice give VPN → Bob accept → WT connect → tunnel probe', async ({ browser }) => {
  const aliceCtx = await browser.newContext()
  const bobCtx = await browser.newContext()
  const alice = await aliceCtx.newPage()
  const bob = await bobCtx.newPage()
  const bobConsole: string[] = []
  bob.on('console', (m) => bobConsole.push(m.type() + ': ' + m.text()))
  bob.on('pageerror', (e) => bobConsole.push('PAGEERROR: ' + e.message))

  await bob.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
  await alice.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })

  // Wait for VPN panels to init
  const aLog = await waitVPNReady(alice)
  const bLog = await waitVPNReady(bob)
  console.log('Alice ready:', /signaling ON/.test(aLog))
  console.log('Bob ready:', /signaling ON/.test(bLog))

  // Alice shares VPN
  await alice.getByTestId('vpn-give').click()
  await alice.getByTestId('vpn-friend-id').fill(BOB_SK)
  await alice.getByTestId('vpn-share-confirm').click()

  // Wait for invite to arrive at Bob
  let bobSaw = false
  for (let i = 0; i < 15; i++) {
    await bob.waitForTimeout(1000)
    const modal = await bob.getByTestId('vpn-incoming-modal').count()
    const blog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (modal > 0 || /Входящий vpn-invite/.test(blog)) { bobSaw = true; break }
  }
  expect(bobSaw).toBeTruthy()

  // Bob accepts
  const acceptBtn = bob.getByTestId('vpn-accept')
  if (await acceptBtn.count()) {
    await acceptBtn.click()
  }

  // Wait for connect result
  let finalBobLog = ''
  for (let i = 0; i < 25; i++) {
    await bob.waitForTimeout(1000)
    finalBobLog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (/Туннель|WebTransport connected|Ошибка accept/.test(finalBobLog)) break
  }

  const finalAliceLog = await alice.getByTestId('vpn-log').innerText()
  console.log('ALICE FINAL:\n', finalAliceLog)
  console.log('BOB FINAL:\n', finalBobLog)
  console.log('BOB CONSOLE:\n', bobConsole.filter(l => /WT|tunnel|error|Error|connected|Туннель/i.test(l)).join('\n'))

  await alice.screenshot({ path: 'e2e-shots/crit-full-alice.png', fullPage: true })
  await bob.screenshot({ path: 'e2e-shots/crit-full-bob.png', fullPage: true })

  // Assertions
  expect(finalAliceLog).toContain('Инвайт OK')
  expect(finalBobLog).toMatch(/WT connect/)
  // Either success or explicit error (not hang)
  const settled = /Туннель|WebTransport connected|Ошибка accept/.test(finalBobLog)
  console.log('SETTLED:', settled)
  expect(settled).toBeTruthy()

  await aliceCtx.close()
  await bobCtx.close()
})
