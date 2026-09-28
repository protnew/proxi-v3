import { test, expect } from '@playwright/test'

const APP_URL = 'http://localhost:4173'

test('два браузера соединяются через WebRTC', async ({ browser }) => {
  const ctx1 = await browser.newContext()
  const ctx2 = await browser.newContext()
  const p1 = await ctx1.newPage()
  const p2 = await ctx2.newPage()

  p1.on('console', msg => console.log(`  [A] ${msg.type()}: ${msg.text()}`))
  p2.on('console', msg => console.log(`  [B] ${msg.type()}: ${msg.text()}`))
  p1.on('pageerror', err => console.log(`  [A ERROR] ${err.message}`))
  p2.on('pageerror', err => console.log(`  [B ERROR] ${err.message}`))

  // 1. Открываем обе страницы
  await p1.goto(APP_URL)
  await p2.goto(APP_URL)

  // 1b. Debug: dump HTML
  const html = await p1.content()
  console.log('HTML snippet:', html.slice(0, 500))
  const hasApp = await p1.locator('#app').innerHTML()
  console.log('#app innerHTML:', hasApp.slice(0, 200))

  // 2. Ждём рендеринг и нажимаем кнопки
  await p1.waitForTimeout(2000)
  await p1.screenshot({ path: 'test-results/page1.png' })
  await p2.screenshot({ path: 'test-results/page2.png' })

  // Кнопки содержат emoji — используем более точный селектор
  const btnA = p1.locator('.btn-a')
  const btnB = p2.locator('.btn-b')
  await expect(btnA).toBeVisible({ timeout: 10000 })
  await expect(btnB).toBeVisible({ timeout: 10000 })
  await btnA.click()
  await btnB.click()
  await p1.waitForTimeout(3000)

  // 3. Достаём полные ключи из window.__testKey
  const keyA = await p1.evaluate(() => (window as any).__testKey as string)
  const keyB = await p2.evaluate(() => (window as any).__testKey as string)
  console.log(`🔑 A: ${keyA?.slice(0, 16)}...`)
  console.log(`🔑 B: ${keyB?.slice(0, 16)}...`)
  expect(keyA).toBeTruthy()
  expect(keyB).toBeTruthy()
  expect(keyA).not.toBe(keyB)

  // 4. Side A = exit node (toggle checkbox)
  await p1.click('input[type="checkbox"]')

  // 5. Обмениваемся ключами — вставляем в инпуты
  await p1.fill('input[placeholder="Вставь ключ с другой стороны"]', keyB)
  await p2.fill('input[placeholder="Вставь ключ с другой стороны"]', keyA)

  // 6. Both subscribe first, THEN A sends offer
  await p2.click('button:has-text("Готов")')
  await p1.waitForTimeout(2000)
  await p1.click('button:has-text("Подключиться")')
  console.log('🚀 B subscribed, A initiating offer')

  // 7. Ждём TUNNEL CONNECTED (до 30 сек)
  console.log('⏳ Waiting for tunnel...')
  
  try {
    await Promise.all([
      p1.locator('text=TUNNEL CONNECTED').waitFor({ timeout: 30000 }),
      p2.locator('text=TUNNEL CONNECTED').waitFor({ timeout: 30000 }),
    ])
    console.log('✅ TUNNEL CONNECTED on both sides!')
  } catch (e) {
    // Dump log for debugging
    const logA = await p1.locator('.logbox').textContent()
    const logB = await p2.locator('.logbox').textContent()
    console.log('\n📋 Log A:', logA)
    console.log('\n📋 Log B:', logB)
    throw e
  }

  // 8. Side B test fetch through VPN
  await p2.waitForTimeout(2000)
  const fetchBtn = p2.locator('button:has-text("Загрузить")')
  if (await fetchBtn.isVisible().catch(() => false)) {
    await fetchBtn.click()
    await p2.waitForTimeout(10000)
    const result = await p2.locator('.res').textContent().catch(() => '')
    console.log(`📥 Fetch result: ${result?.slice(0, 100) || '(none)'}`)
  } else {
    console.log('⚠️ Fetch button not visible (exit node toggle issue)')
  }

  await ctx1.close()
  await ctx2.close()
  console.log('\n🎉 TEST PASSED')
})
