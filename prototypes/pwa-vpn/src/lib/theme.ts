/**
 * Theme manager — light/dark mode
 * Persists preference to localStorage
 */

export type Theme = 'dark' | 'light'

let currentTheme: Theme = 'dark'
const listeners = new Set<(theme: Theme) => void>()

export function getTheme(): Theme { return currentTheme }

export function setTheme(theme: Theme): Theme {
  currentTheme = theme
  localStorage.setItem('messenger-theme', theme)
  applyTheme(theme)
  listeners.forEach(cb => cb(theme))
  return theme
}

export function toggleTheme(): Theme {
  return currentTheme === 'dark' ? setTheme('light') : setTheme('dark')
}

export function initTheme() {
  const stored = localStorage.getItem('messenger-theme') as Theme | null
  if (stored === 'light' || stored === 'dark') currentTheme = stored
  applyTheme(currentTheme)
}

export function onThemeChange(cb: (theme: Theme) => void) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

function applyTheme(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme)
}
