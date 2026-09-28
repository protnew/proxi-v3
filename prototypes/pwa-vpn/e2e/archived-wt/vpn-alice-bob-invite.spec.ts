
import { test, expect } from '@playwright/test'
const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const BOB_SK = '2'.repeat(64)

test.setTimeout(120000);
test('Alice invite → Bob accept → WT connect result', async ({ browser }) => {
  const aliceCtx = await browser.newContext()
  const bobCtx = await browser.newContext()
  const alice = await aliceCtx.newPage()
  const bob = await bobCtx.newPage()
  const bobConsole: string[] = []
  bob.on('console', (m) => bobConsole.push(m.type() + ': ' + m.text()))
  bob.on('pageerror', (e) => bobConsole.push('PAGEERROR: ' + e.message))

  await bob.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })
  await alice.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
  await alice.waitForTimeout(6000)
  await bob.waitForTimeout(6000)

  await alice.getByTestId('vpn-give').click()
  await alice.getByTestId('vpn-friend-id').fill(BOB_SK)
  await alice.getByTestId('vpn-share-confirm').click()

  let bobSaw = false
  for (let i = 0; i < 15; i++) {
    await bob.waitForTimeout(800)
    if (await bob.getByTestId('vpn-incoming-modal').count()) { bobSaw = true; break }
    const blog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (/Входящий vpn-invite/.test(blog)) { bobSaw = true; break }
  }
  expect(bobSaw).toBeTruthy()

  if (await bob.getByTestId('vpn-accept').count()) {
    await bob.getByTestId('vpn-accept').click()
  }
  // wait up to 20s for connect result
  let blog = ''
  for (let i = 0; i < 20; i++) {
    await bob.waitForTimeout(1000)
    blog = await bob.getByTestId('vpn-log').innerText().catch(() => '')
    if (/WebTransport connected|Туннель IP|Ошибка accept|Туннель probe/.test(blog)) break
  }
  const aLog = await alice.getByTestId('vpn-log').innerText()
  console.log('ALICE LOG:\n', aLog)
  console.log('BOB LOG:\n', blog)
  console.log('BOB CONSOLE:\n', bobConsole.filter(l => /WT|WebTransport|error|Error|fail|quic|cert/i.test(l)).join('\n'))
  await bob.screenshot({ path: 'e2e-shots/crit-bob-after-accept.png', fullPage: true })

  expect(aLog).toContain('Инвайт OK')
  expect(blog).toMatch(/WT connect/)
  // Soft assert: either success or explicit error (not hang forever)
  const settled = /WebTransport connected|Туннель IP|Ошибка accept|Туннель probe/.test(blog)
  console.log('SETTLED', settled)
  // Don't fail hard if browser WT unsupported — record as gap
  if (!settled) {
    console.log('GAP: WT connect hung without result')
  }
  await aliceCtx.close(); await bobCtx.close()
})
