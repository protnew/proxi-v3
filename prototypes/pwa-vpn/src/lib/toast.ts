/** INF-001: toast + local error log (SSR-safe) */
export type ToastLevel = 'info' | 'ok' | 'warn' | 'error'

export interface Toast {
  id: string
  level: ToastLevel
  message: string
  at: number
}

const LOG_KEY = 'proxi_error_log'
const MAX_LOG = 200
let listeners: Array<(t: Toast) => void> = []
let memLog: Array<{ at: number; level: string; message: string }> = []

function lsGet(k: string): string | null {
  try { return typeof localStorage !== 'undefined' ? localStorage.getItem(k) : null } catch { return null }
}
function lsSet(k: string, v: string): void {
  try { if (typeof localStorage !== 'undefined') localStorage.setItem(k, v) } catch { /* */ }
}
function lsDel(k: string): void {
  try { if (typeof localStorage !== 'undefined') localStorage.removeItem(k) } catch { /* */ }
}

export function onToast(fn: (t: Toast) => void): () => void {
  listeners.push(fn)
  return () => { listeners = listeners.filter(x => x !== fn) }
}

export function toast(message: string, level: ToastLevel = 'info'): Toast {
  const t: Toast = { id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`, level, message, at: Date.now() }
  for (const fn of listeners) {
    try { fn(t) } catch { /* ignore */ }
  }
  if (level === 'error' || level === 'warn') {
    appendErrorLog({ at: t.at, level, message })
  }
  return t
}

export function appendErrorLog(entry: { at: number; level: string; message: string }): void {
  memLog.push(entry)
  while (memLog.length > MAX_LOG) memLog.shift()
  try {
    const raw = lsGet(LOG_KEY)
    const arr = raw ? JSON.parse(raw) as unknown[] : []
    arr.push(entry)
    while (arr.length > MAX_LOG) arr.shift()
    lsSet(LOG_KEY, JSON.stringify(arr))
  } catch { /* */ }
}

export function getErrorLog(): Array<{ at: number; level: string; message: string }> {
  try {
    const raw = lsGet(LOG_KEY)
    if (raw) return JSON.parse(raw)
  } catch { /* */ }
  return [...memLog]
}

export function exportErrorLog(): string {
  return JSON.stringify(getErrorLog(), null, 2)
}

export function clearErrorLog(): void {
  memLog = []
  lsDel(LOG_KEY)
}
