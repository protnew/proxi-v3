/**
 * PUSH-001: Service Worker push handler structure test
 */
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const swCode = readFileSync(resolve(__dirname, '../public/sw.js'), 'utf-8')

describe('PUSH-001 Service Worker', () => {
  it('registers push event listener', () => {
    expect(swCode).toContain("addEventListener('push'")
  })

  it('registers notificationclick listener', () => {
    expect(swCode).toContain("addEventListener('notificationclick'")
  })

  it('has cache version for INF-003 update', () => {
    expect(swCode).toContain('SW_VERSION')
    expect(swCode).toContain('CACHE_NAME')
  })

  it('handles push data json and text', () => {
    expect(swCode).toContain('event.data.json()')
    expect(swCode).toContain('event.data.text()')
  })

  it('shows notification with actions', () => {
    expect(swCode).toContain("showNotification")
    expect(swCode).toContain("action: 'open'")
  })

  it('skipWaiting for update flow', () => {
    expect(swCode).toContain('skipWaiting')
  })

  it('message handler for INF-003 version check', () => {
    expect(swCode).toContain("GET_VERSION")
  })
})
