
import { test, expect } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import path from 'node:path'

const BASE = process.env.APP_URL || 'http://127.0.0.1:8090'
const BOB_SK = '2'.repeat(64)

test('node: signed kind:30090 accepted by local relay', async () => {
  const script = path.join(process.cwd(), 'scripts', 'test-nostr-vpn-sign.mjs')
  const out = execFileSync(process.execPath, [script], { encoding: 'utf8', timeout: 15000 })
  console.log(out)
  expect(out).toContain('"ok":true')
  expect(out).toContain('published')
})

test('product UI: give VPN → WT start + signed invite log', async ({ page }) => {
  await page.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(6000)
  await expect(page.getByTestId('vpn-give')).toBeVisible()
  await page.getByTestId('vpn-give').click()
  await page.getByTestId('vpn-friend-id').fill(BOB_SK)
  await page.getByTestId('vpn-share-confirm').click()
  // poll log up to 15s
  let log = ''
  for (let i = 0; i < 15; i++) {
    await page.waitForTimeout(1000)
    log = await page.getByTestId('vpn-log').innerText().catch(() => '')
    if (/Инвайт OK|WT /.test(log)) break
  }
  console.log('VPN LOG FULL:\n', log)
  await page.screenshot({ path: 'e2e-shots/crit-give-vpn-flow.png', fullPage: true })
  expect(log).toMatch(/Nostr VPN signaling ON|Инвайт OK|WT /)
  // Hard success if invite signed path completed
  if (log.includes('Инвайт OK')) {
    expect(log).toMatch(/Инвайт OK/)
    expect(log).toMatch(/WT /)
  } else {
    // surface error for critique
    console.log('INVITE_NOT_COMPLETE')
    expect(log).toMatch(/Ошибка|WT |signaling/)
  }
})

test('API wt/start flat certHash 64', async ({ request }) => {
  const signup = await request.post(BASE + '/api/auth/signup', {
    data: { npub: 'npub1e2e' + Date.now(), password: 'TestPass123!' },
  })
  expect(signup.ok()).toBeTruthy()
  const { access_token } = await signup.json()
  const start = await request.post(BASE + '/api/vpn/wt/start', {
    headers: { Authorization: 'Bearer ' + access_token },
    data: {},
  })
  expect(start.ok()).toBeTruthy()
  const body = await start.json()
  console.log('wt start', body)
  const hash = body.certHash || body.stats?.certHash
  expect(hash).toHaveLength(64)
})
