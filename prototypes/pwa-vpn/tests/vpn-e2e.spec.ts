import { test, expect, type Page } from '@playwright/test'

const APP_URL = 'http://localhost:4173'

async function login(page: Page, label: string) {
  await page.goto(APP_URL)
  await page.waitForTimeout(3000)

  // Проверяем — может уже залогинен (localStorage из того же context)
  const vpnVisible = await page.locator('[data-testid="vpn-product"]').isVisible().catch(() => false)
  if (vpnVisible) {
    await page.evaluate(() => {
      document.querySelectorAll('.demo-panel, .demo-header').forEach(p => (p as HTMLElement).style.display = 'none')
    })
    console.log(`✅ ${label} already logged in`)
    return
  }

  // Проверяем auth screen
  const authVisible = await page.locator('[data-testid="auth-screen"]').isVisible().catch(() => false)
  if (!authVisible) {
    // Wait more and retry
    await page.waitForSelector('[data-testid="auth-screen"], [data-testid="vpn-product"]', { timeout: 15000 })
    const vpn2 = await page.locator('[data-testid="vpn-product"]').isVisible().catch(() => false)
    if (vpn2) {
      await page.evaluate(() => {
        document.querySelectorAll('.demo-panel, .demo-header').forEach(p => (p as HTMLElement).style.display = 'none')
      })
      console.log(`✅ ${label} already logged in (slow render)`)
      return
    }
  }

  await page.click('[data-testid="auth-create"]')
  await page.waitForTimeout(2000)
  const nsec = await page.inputValue('[data-testid="auth-nsec"]').catch(() => '')
  await page.check('[data-testid="auth-backup-ok"]')
  await Promise.all([
    page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {}),
    page.click('[data-testid="auth-enter"]'),
  ])
  await page.waitForTimeout(3000)

  const authVisible2 = await page.locator('[data-testid="auth-screen"]').isVisible().catch(() => false)
  if (authVisible2) {
    await page.click('[data-testid="auth-import-open"]')
    await page.fill('[data-testid="auth-import-input"]', nsec)
    await page.click('[data-testid="auth-import"]')
    await page.waitForTimeout(2000)
    const backupVisible = await page.locator('[data-testid="auth-backup-ok"]').isVisible().catch(() => false)
    if (backupVisible) {
      await page.check('[data-testid="auth-backup-ok"]')
      await Promise.all([
        page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {}),
        page.click('[data-testid="auth-enter"]'),
      ])
      await page.waitForTimeout(3000)
    }
  }

  await page.waitForSelector('[data-testid="vpn-product"]', { timeout: 20000 })
  await page.evaluate(() => {
    document.querySelectorAll('.demo-panel, .demo-header').forEach(p => (p as HTMLElement).style.display = 'none')
  })
  console.log(`✅ ${label} logged in`)
}

async function clickTestId(page: Page, testid: string) {
  await page.evaluate((id) => {
    const el = document.querySelector(`[data-testid="${id}"]`) as HTMLElement
    if (el) el.click()
  }, testid)
}

test('VPN E2E: WebRTC P2P туннель', async ({ browser }) => {
  const ctx = await browser.newContext()
  const p1 = await ctx.newPage()
  const p2 = await ctx.newPage()

  p1.on('console', msg => {
    const t = msg.text()
    if (t.includes('connected') || t.includes('VPN') || t.includes('tunnel') || t.includes('TUNNEL'))
      console.log(`  [A] ${t.slice(0, 150)}`)
  })
  p2.on('console', msg => {
    const t = msg.text()
    if (t.includes('connected') || t.includes('VPN') || t.includes('tunnel') || t.includes('TUNNEL'))
      console.log(`  [B] ${t.slice(0, 150)}`)
  })

  await login(p1, 'Alice')
  await login(p2, 'Bob')

  await clickTestId(p1, 'vpn-tab-host')
  console.log('Alice: host started')
  await p1.waitForTimeout(3000)

  await clickTestId(p2, 'vpn-tab-join')
  console.log('Bob: joiner started')

  let connected = false
  for (let i = 0; i < 20; i++) {
    await p1.waitForTimeout(2000)
    const statusA = await p1.locator('[data-testid="vpn-tab-p2p-status"]').textContent().catch(() => '')
    const statusB = await p2.locator('[data-testid="vpn-tab-p2p-status"]').textContent().catch(() => '')

    if ((statusA || '').includes('connected') || (statusB || '').includes('connected')) {
      connected = true
      console.log(`  ✅ TUNNEL CONNECTED after ${(i+1)*2}s`)
      console.log(`  A: ${(statusA||'').slice(0, 150)}`)
      console.log(`  B: ${(statusB||'').slice(0, 150)}`)
      break
    }
    if (i % 3 === 0)
      console.log(`  ${(i+1)*2}s: A=${(statusA||'').slice(0, 50)} | B=${(statusB||'').slice(0, 50)}`)
  }

  await p1.screenshot({ path: 'test-results/vpn-e2e-host.png' })
  await p2.screenshot({ path: 'test-results/vpn-e2e-joiner.png' })

  const logA = await p1.locator('[data-testid="vpn-log"]').textContent().catch(() => '')
  const logB = await p2.locator('[data-testid="vpn-log"]').textContent().catch(() => '')
  console.log(`  Log A: ${(logA || '').slice(0, 250)}`)
  console.log(`  Log B: ${(logB || '').slice(0, 250)}`)

  expect(connected, 'WebRTC tunnel should connect').toBeTruthy()

  await ctx.close()
  console.log('✅ VPN E2E PASSED')
})
