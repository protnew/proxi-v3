/**
 * Offline message outbox — queue when transports down, flush when online.
 * SL-053: Offline queued messages via Nostr outbox pattern.
 *
 * Survives reload via localStorage. Flush order = FIFO.
 */

export interface OutboxItem {
  id: string
  to: string
  text: string
  createdAt: number
  attempts: number
  lastError?: string
}

const KEY = 'indestructible-outbox'
const MAX_ITEMS = 200
const MAX_ATTEMPTS = 8

type Sender = (to: string, text: string) => Promise<{ ok: boolean; via?: string; error?: string }>

let memoryStore: OutboxItem[] = []

function load(): OutboxItem[] {
  if (typeof localStorage === 'undefined') return [...memoryStore]
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return [...memoryStore]
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr : [...memoryStore]
  } catch {
    return [...memoryStore]
  }
}

function save(items: OutboxItem[]) {
  const clipped = items.slice(-MAX_ITEMS)
  memoryStore = clipped
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.setItem(KEY, JSON.stringify(clipped))
  } catch { /* quota / private mode */ }
}

export function listOutbox(): OutboxItem[] {
  return load()
}

export function enqueue(to: string, text: string): OutboxItem {
  const items = load()
  const item: OutboxItem = {
    id: (typeof crypto !== 'undefined' && crypto.randomUUID)
      ? crypto.randomUUID()
      : `obx-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
    to,
    text,
    createdAt: Date.now(),
    attempts: 0,
  }
  items.push(item)
  save(items)
  console.log('[outbox] queued', item.id.slice(0, 8), 'to', to.slice(0, 8))
  return item
}

export function remove(id: string) {
  save(load().filter(i => i.id !== id))
}

export function clearOutbox() {
  save([])
}

/**
 * Flush queue using provided sender. Stops on first hard failure sequence.
 * Returns count sent.
 */
export async function flushOutbox(send: Sender): Promise<{ sent: number; left: number }> {
  const items = load()
  if (!items.length) return { sent: 0, left: 0 }

  let sent = 0
  const remain: OutboxItem[] = []

  for (const item of items) {
    if (item.attempts >= MAX_ATTEMPTS) {
      console.warn('[outbox] drop after max attempts', item.id)
      continue
    }
    try {
      const r = await send(item.to, item.text)
      if (r.ok) {
        sent++
        console.log('[outbox] flushed', item.id.slice(0, 8), 'via', r.via || 'ok')
      } else {
        item.attempts++
        item.lastError = r.error || 'send failed'
        remain.push(item)
      }
    } catch (e: any) {
      item.attempts++
      item.lastError = e?.message || String(e)
      remain.push(item)
    }
  }

  save(remain)
  return { sent, left: remain.length }
}

/** Auto-flush on online + interval */
export function startOutboxWatcher(send: Sender, intervalMs = 15000): () => void {
  const tick = () => { flushOutbox(send).catch(() => {}) }
  const onOnline = () => tick()
  if (typeof window !== 'undefined') {
    window.addEventListener('online', onOnline)
  }
  const id = setInterval(tick, intervalMs)
  // first try soon
  setTimeout(tick, 2000)
  return () => {
    clearInterval(id)
    if (typeof window !== 'undefined') window.removeEventListener('online', onOnline)
  }
}
