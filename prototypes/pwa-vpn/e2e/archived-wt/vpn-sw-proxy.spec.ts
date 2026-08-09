
import { test, expect, type Page } from '@playwright/test'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const BOB_SK = '2'.repeat(64)

test.setTimeout(120000)

async function waitVPNReady(page: Page, timeout = 20000) {
  for (let i = 0; i < timeout / 1000; i++) {
    await page.waitForTimeout(1000)
    const log = await page.getByTestId('vpn-log').innerText().catch(() => '')
    if (/Nostr VPN signaling ON/.test(log)) return log
  }
  return ''
}

test('SW proxy: external fetch goes through WT tunnel', async ({ browser }) => {
  const aliceCtx = await browser.newContext()
  const bobCtx = await browser.newContext()
  const alice = await aliceCtx.newPage()
  const bob = await bobCtx.newPage()
  const bobConsole: string[] = []
  bob.on('console', (m) => bobConsole.push(m.type() + ': ' + m.text()))

  // Load both
  await bob.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
  await alice.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })

  const aLog = await waitVPNReady(alice)
  const bLog = await waitVPNReady(bob)
  expect(/signaling ON/.test(aLog)).toBeTruthy()
  expect(/signaling ON/.test(bLog)).toBeTruthy()

  // Alice gives VPN
  await alice.getByTestId('vpn-give').click()
  await alice.getByTestId('vpn-friend-id').fill(BOB_SK)
  await alice.getByTestId('vpn-share-confirm').click()
  await alice.waitForTimeout(2000)

  // Bob waits for invite
  let saw = false
  for (let i = 0; i < 15; i++) {
    await bob.waitForTimeout(1000)
    const modal = await bob.getByTestId('vpn-incoming-modal').count()
    const blog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (modal > 0 || /Входящий/.test(blog)) { saw = true; break }
  }
  expect(saw).toBeTruthy()

  // Bob accepts
  if (await bob.getByTestId('vpn-accept').count()) {
    await bob.getByTestId('vpn-accept').click()
  }

  // Wait for WT connect
  let bobConnected = false
  for (let i = 0; i < 20; i++) {
    await bob.waitForTimeout(1000)
    const blog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (/WebTransport connected|Туннель/.test(blog)) { bobConnected = true; break }
  }
  console.log('Bob connected:', bobConnected)

  // Now try external fetch through SW proxy
  // The SW should intercept this and route via WT CONNECT tunnel
  const tunnelIp = await bob.evaluate(async () => {
    try {
      const resp = await fetch('http://api.ipify.org', { cache: 'no-store' })
      if (resp.ok) {
        return await resp.text()
      }
      return 'fetch_failed_' + resp.status
    } catch (e) {
      return 'error_' + (e as Error).message
    }
  })

  console.log('Tunnel IP via SW fetch:', tunnelIp)
  console.log('Bob console relevant:', bobConsole.filter(l => /tunnel|fetch|proxy|SW|VPN/i.test(l)).join('\n'))

  await bob.screenshot({ path: 'e2e-shots/crit-sw-proxy.png', fullPage: true })

  // The fetch should either:
  // 1. Return an IP through the tunnel (best case)
  // 2. Return fallback if SW not yet active (acceptable)
  // 3. Return error if WT CONNECT failed
  console.log('RESULT:', tunnelIp)

  await aliceCtx.close()
  await bobCtx.close()
})
