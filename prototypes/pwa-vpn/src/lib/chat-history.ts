/**
 * MSG-102: Chat history — IndexedDB-backed local storage
 * Survives page reload, no server needed
 */

const DB_NAME = 'proxi-chat-history'
const DB_VERSION = 1
const STORE = 'messages'

let dbPromise: Promise<IDBDatabase> | null = null

function openDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB not available'))
      return
    }
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: ['chatId', 'id'] })
        store.createIndex('chatId', 'chatId', { unique: false })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return dbPromise
}

export interface StoredMessage {
  chatId: string
  id: string
  from: string
  to: string
  text: string
  timestamp: number
  type: string
  read: boolean
}

export async function saveMessage(chatId: string, msg: StoredMessage): Promise<void> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).put({ ...msg, chatId })
    await tx.done
  } catch (e) {
    console.warn('[chat-history] save failed', e)
  }
}

export async function loadMessages(chatId: string, limit = 100): Promise<StoredMessage[]> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE, 'readonly')
    const store = tx.objectStore(STORE)
    const idx = store.index('chatId')
    const range = IDBKeyRange.only(chatId)
    const results = await new Promise<StoredMessage[]>((res, rej) => {
      const r = idx.getAll(range)
      r.onsuccess = () => res(r.result as StoredMessage[])
      r.onerror = () => rej(r.error)
    })
    return results.sort((a, b) => a.timestamp - b.timestamp).slice(-limit)
  } catch (e) {
    // Fallback: manual filter
    try {
      const db = await openDB()
      const tx = db.transaction(STORE, 'readonly')
      const all = await new Promise<StoredMessage[]>((res, rej) => {
        const r = tx.objectStore(STORE).getAll()
        r.onsuccess = () => res(r.result as StoredMessage[])
        r.onerror = () => rej(r.error)
      })
      return all.filter(m => m.chatId === chatId).sort((a,b) => a.timestamp - b.timestamp).slice(-limit)
    } catch {
      return []
    }
  }
}

export async function loadAllChatIds(): Promise<string[]> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE, 'readonly')
    const all = await new Promise<StoredMessage[]>((res, rej) => {
      const r = tx.objectStore(STORE).getAll()
      r.onsuccess = () => res(r.result as StoredMessage[])
      r.onerror = () => rej(r.error)
    })
    return [...new Set(all.map(m => m.chatId))]
  } catch {
    return []
  }
}

export async function clearHistory(chatId?: string): Promise<void> {
  try {
    const db = await openDB()
    const tx = db.transaction(STORE, 'readwrite')
    if (chatId) {
      const idx = tx.objectStore(STORE).index('chatId')
      const range = IDBKeyRange.only(chatId)
      const all = await new Promise<StoredMessage[]>((res, rej) => {
        const r = idx.getAll(range)
        r.onsuccess = () => res(r.result as StoredMessage[])
        r.onerror = () => rej(r.error)
      })
      for (const m of all) {
        tx.objectStore(STORE).delete([m.chatId, m.id])
      }
    } else {
      tx.objectStore(STORE).clear()
    }
    await tx.done
  } catch (e) {
    console.warn('[chat-history] clear failed', e)
  }
}
