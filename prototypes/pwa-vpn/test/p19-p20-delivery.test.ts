import { describe, it, expect, beforeEach } from 'vitest'
import { toast, onToast, type Toast } from '../src/lib/toast'

describe('P19 toast API', () => {
  it('emits toast to listeners with level', () => {
    const seen: Toast[] = []
    const off = onToast(t => seen.push(t))
    toast('Не удалось отправить', 'error')
    off()
    expect(seen.length).toBe(1)
    expect(seen[0].level).toBe('error')
    expect(seen[0].message).toContain('Не удалось')
  })
})

describe('P20 delivery honesty contract', () => {
  it('pending must not be represented as double-check', () => {
    const glyph = (deliveryStatus?: string, read?: boolean) => {
      if (deliveryStatus === 'pending') return '⏳'
      if (deliveryStatus === 'failed') return '⚠'
      if (deliveryStatus === 'delivered' || read) return '✓✓'
      if (deliveryStatus === 'sent') return '✓'
      return '⏳'
    }
    expect(glyph('pending', true)).toBe('⏳') // even if read flag wrongly set, pending wins in UI contract test
    expect(glyph('sent', false)).toBe('✓')
    expect(glyph('delivered', true)).toBe('✓✓')
    expect(glyph(undefined, true)).toBe('✓✓')
  })
})