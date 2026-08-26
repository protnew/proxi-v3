/**
 * VPN-101 Playwright: two pages same origin → DataChannel connected
 */
import { test, expect } from '@playwright/test'

const BASE = process.env.PROXI_URL || 'http://127.0.0.1:8090'

test.describe('VPN-101 two-tab P2P', () => {
  test.setTimeout(90000)

  test('host + joiner DataChannel open', async ({ browser }) => {
    const ctx = await browser.newContext()
    const host = await ctx.newPage()
    const joiner = await ctx.newPage()

    await host.goto(BASE + '/', { waitUntil: 'domcontentloaded' })
    await joiner.goto(BASE + '/', { waitUntil: 'domcontentloaded' })

    // wait loading overlay hidden
    await host.waitForFunction(() => {
      const el = document.getElementById('loading-overlay')
      return !el || el.classList.contains('hidden') || el.style.display === 'none'
    }, { timeout: 20000 }).catch(() => {})
    await joiner.waitForFunction(() => {
      const el = document.getElementById('loading-overlay')
      return !el || el.classList.contains('hidden') || el.style.display === 'none'
    }, { timeout: 20000 }).catch(() => {})

    // buttons may be below fold
    const hostBtn = host.getByTestId('vpn-tab-host')
    const joinBtn = joiner.getByTestId('vpn-tab-join')
    await hostBtn.scrollIntoViewIfNeeded()
    await joinBtn.scrollIntoViewIfNeeded()
    await expect(hostBtn).toBeVisible({ timeout: 15000 })
    await expect(joinBtn).toBeVisible({ timeout: 15000 })

    await hostBtn.click()
    await joinBtn.click()

    await expect(host.getByTestId('vpn-tab-p2p-status')).toContainText('connected', { timeout: 45000 })
    await expect(joiner.getByTestId('vpn-tab-p2p-status')).toContainText('connected', { timeout: 45000 })

    // screenshot proof
    await host.screenshot({ path: 'test-results/vpn101-host.png', fullPage: true })
    await joiner.screenshot({ path: 'test-results/vpn101-joiner.png', fullPage: true })

    await ctx.close()
  })
})
