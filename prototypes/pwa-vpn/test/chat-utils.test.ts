import { describe, it, expect } from 'vitest'
import { formatTime, formatDay, getDate, formatSize, extractUrls } from '../src/lib/chat-utils'

describe('chat-utils', () => {
  it('formatTime returns time string', () => {
    const s = formatTime(Date.UTC(2026, 0, 15, 12, 30))
    expect(typeof s).toBe('string')
    expect(s.length).toBeGreaterThan(0)
  })

  it('formatDay returns non-empty', () => {
    expect(formatDay(Date.now()).length).toBeGreaterThan(0)
  })

  it('getDate today/yesterday labels', () => {
    expect(getDate(Date.now())).toBe('Сегодня')
    const y = new Date()
    y.setDate(y.getDate() - 1)
    expect(getDate(y.getTime())).toBe('Вчера')
  })

  it('formatSize units', () => {
    expect(formatSize(500)).toBe('500 B')
    expect(formatSize(2048)).toContain('KB')
    expect(formatSize(5 * 1024 * 1024)).toContain('MB')
  })

  it('extractUrls finds http(s)', () => {
    const urls = extractUrls('see https://example.com and http://a.b/c')
    expect(urls).toContain('https://example.com')
    expect(urls.some(u => u.startsWith('http://'))).toBe(true)
    expect(extractUrls('no links')).toEqual([])
  })
})
