/**
 * VPN-101 + VPN-105 Playwright:
 * two pages → DataChannel connected → HTTP GET via tunnel (ipify)
 */
import { test, expect } from '@playwright/test'

const BASE = process.env.PROXI_URL || process.env.APP_URL || 'http://127.0.0.1:8090'

test.describe('VPN-101/105 two-tab P2P + tunnel HTTP', () => {
  test.setTimeout(120000)

  test('host + joiner DC open and tunnel HTTP', async ({ browser }) => {
    const ctx = await browser.newContext()
    const host = await ctx.newPage()
    const joiner = await ctx.newPage()

    await host.goto(BASE + '/?role=alice', { waitUntil: 'domcontentloaded' })
    await joiner.goto(BASE + '/?role=bob', { waitUntil: 'domcontentloaded' })

    for (const p of [host, joiner]) {
      await p.waitForFunction(() => {
        const el = document.getElementById('loading-overlay')
        return !el || el.classList.contains('hidden') || getComputedStyle(el).display === 'none'
      }, { timeout: 20000 }).catch(() => {})
      await p.evaluate(() => {
        document.querySelectorAll('.demo-panel').forEach(el => {
          ;(el as HTMLElement).style.display = 'none'
        })
      })
    }

    await host.getByTestId('vpn-tab-host').click({ force: true })
    await joiner.getByTestId('vpn-tab-join').click({ force: true })

    await expect(host.getByTestId('vpn-tab-p2p-status')).toContainText('connected', { timeout: 45000 })
    await expect(joiner.getByTestId('vpn-tab-p2p-status')).toContainText('connected', { timeout: 45000 })

    // VPN-105: joiner fetches public IP through host exit via DataChannel
    // status shows "IP x.x.x.x" when tunnel HTTP works
    await expect(joiner.getByTestId('vpn-tab-p2p-status')).toContainText(/IP\s+\d+\.\d+\.\d+\.\d+/, {
      timeout: 30000,
    })

    await host.screenshot({ path: 'test-results/vpn101-host.png', fullPage: true })
    await joiner.screenshot({ path: 'test-results/vpn105-joiner-tunnel-ip.png', fullPage: true })

    await ctx.close()
  })
})
