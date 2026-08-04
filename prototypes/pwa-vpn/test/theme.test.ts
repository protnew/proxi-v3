/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { getTheme, setTheme, toggleTheme, initTheme } from '../src/lib/theme'

describe('theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
    setTheme('dark')
  })

  it('defaults dark after set', () => {
    expect(getTheme()).toBe('dark')
  })

  it('setTheme persists and applies attr', () => {
    setTheme('light')
    expect(getTheme()).toBe('light')
    expect(localStorage.getItem('messenger-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('toggleTheme flips', () => {
    setTheme('dark')
    const n = toggleTheme()
    expect(n).toBe('light')
    expect(toggleTheme()).toBe('dark')
  })

  it('initTheme reads storage', () => {
    localStorage.setItem('messenger-theme', 'light')
    initTheme()
    expect(getTheme()).toBe('light')
  })
})
