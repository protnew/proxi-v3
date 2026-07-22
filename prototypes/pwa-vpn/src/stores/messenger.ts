/**
 * Messenger state — Svelte 5 stores
 * Using writable stores with get() helper for runes mode
 */
import { writable, derived, get } from 'svelte/store'

// --- Types ---
export interface Profile {
  pubkey: string
  name: string
  about: string
  avatar: string
}

export interface Contact {
  pubkey: string
  name: string
  avatar: string
  isOnline: boolean
  lastSeen: number
}

export interface Message {
  id: string
  from: string
  to: string
  text: string
  timestamp: number
  type: 'text' | 'voice' | 'file' | 'image' | 'system'
  read: boolean
  fileName?: string
  fileSize?: number
  fileUrl?: string
  voiceDuration?: number
  replyTo?: string
  forwardedFrom?: string
  edited?: boolean
  reactions?: Record<string, string[]>
}

export interface Chat {
  id: string
  name: string
  avatar: string
  type: 'dm' | 'group'
  messages: Message[]
  unread: number
  lastActivity: number
  typing: string[]
  members?: string[]
}

export interface ChatView extends Chat {
  lastMessage: Message | null
}

// --- Stores ---
export const profile = writable<Profile>({ pubkey: '', name: 'User', about: '', avatar: '👤' })
export const contacts = writable<Contact[]>([])
export const chats = writable<Chat[]>([])
export const activeChatId = writable<string | null>(null)
export const showNewChat = writable(false)
export const showGroupCreate = writable(false)
export const showSettings = writable(false)

export const connectionStatus = writable<'connected' | 'connecting' | 'error' | 'reconnecting'>('connecting')
export const reconnectCountdown = writable<number>(0)

// --- Derived ---
export const sortedChats = derived(chats, ($chats) =>
  [...$chats]
    .map(c => ({ ...c, lastMessage: c.messages.length > 0 ? c.messages[c.messages.length - 1] : null }))
    .sort((a, b) => b.lastActivity - a.lastActivity)
)

export const activeChat = derived(
  [sortedChats, activeChatId],
  ([$sorted, $id]) => $sorted.find(c => c.id === $id) || null
)

export const totalUnread = derived(chats, ($chats) =>
  $chats.reduce((sum, c) => sum + c.unread, 0)
)

// --- Actions ---
export function ensureDMChat(peerPubkey: string, name: string): string {
  const id = `dm:${peerPubkey}`
  chats.update(cs => {
    if (!Array.isArray(cs)) cs = [];
    if (cs.find(c => c.id === id)) return cs
    return [...cs, {
      id, name, avatar: '👤', type: 'dm',
      messages: [], unread: 0,
      lastActivity: Date.now(), typing: [],
    }]
  })
  addContact(peerPubkey, name)
  return id
}

export function ensureGroupChat(channelId: string, name: string, members: string[] = []): string {
  const id = `group:${channelId}`
  chats.update(cs => {
    if (!Array.isArray(cs)) cs = [];
    if (cs.find(c => c.id === id)) return cs
    return [...cs, {
      id, name, avatar: '👥', type: 'group',
      messages: [], unread: 0,
      lastActivity: Date.now(), typing: [],
      members,
    }]
  })
  return id
}

export function addMessage(chatId: string, msg: Message) {
  chats.update(cs => {
    if (!Array.isArray(cs)) cs = [];
    return cs.map(c => {
      if (c.id !== chatId) return c
      if (c.messages.find(m => m.id === msg.id)) return c
      return {
        ...c,
        messages: [...c.messages, msg],
        lastActivity: msg.timestamp,
        unread: c.unread + (msg.read ? 0 : 1),
      }
    });
  })
}

export function markRead(chatId: string) {
  chats.update(cs => cs.map(c => {
    if (c.id !== chatId) return c
    return { ...c, unread: 0, messages: c.messages.map(m => ({ ...m, read: true })) }
  }))
}

export function editMessage(chatId: string, msgId: string, newText: string) {
  chats.update(cs => cs.map(c => {
    if (c.id !== chatId) return c
    return {
      ...c,
      messages: c.messages.map(m =>
        m.id === msgId ? { ...m, text: newText, edited: true } : m
      ),
    }
  }))
}

export function deleteMessage(chatId: string, msgId: string) {
  chats.update(cs => cs.map(c => {
    if (c.id !== chatId) return c
    return { ...c, messages: c.messages.filter(m => m.id !== msgId) }
  }))
}

export function addReaction(chatId: string, msgId: string, emoji: string, fromPk: string) {
  chats.update(cs => cs.map(c => {
    if (c.id !== chatId) return c
    return {
      ...c,
      messages: c.messages.map(m => {
        if (m.id !== msgId) return m
        const reactions = { ...(m.reactions || {}) }
        if (!reactions[emoji]) reactions[emoji] = []
        if (!reactions[emoji].includes(fromPk)) reactions[emoji].push(fromPk)
        return { ...m, reactions }
      }),
    }
  }))
}

export function setTyping(chatId: string, pubkey: string) {
  chats.update(cs => cs.map(c => {
    if (c.id !== chatId) return c
    if (c.typing.includes(pubkey)) return c
    return { ...c, typing: [...c.typing, pubkey] }
  }))
  setTimeout(() => {
    chats.update(cs => cs.map(c => {
      if (c.id !== chatId) return c
      return { ...c, typing: c.typing.filter(pk => pk !== pubkey) }
    }))
  }, 5000)
}

export function setContactOnline(pk: string, online: boolean) {
  contacts.update(cs => cs.map(c =>
    c.pubkey === pk ? { ...c, isOnline: online, lastSeen: Date.now() } : c
  ))
}

export function addContact(pk: string, name: string) {
  contacts.update(cs => {
    if (!Array.isArray(cs)) cs = [];
    if (cs.find(c => c.pubkey === pk)) return cs
    return [...cs, { pubkey: pk, name, avatar: '👤', isOnline: false, lastSeen: 0 }]
  })
}

// --- Persistence ---
export function saveChats() {
  try { 
    localStorage.setItem('messenger-chats', JSON.stringify(get(chats))) 
  } catch (e: any) {
    if (e.name === 'QuotaExceededError' || (e.message && e.message.includes('Quota'))) {
      console.warn('[messenger] Quota exceeded, trimming old messages...');
      chats.update(cs => cs.map(c => ({
        ...c,
        messages: c.messages.slice(-50)
      })));
      try {
        localStorage.setItem('messenger-chats', JSON.stringify(get(chats)));
      } catch (err) {
        console.error('[messenger] Save failed even after trim', err);
      }
    }
  }
}
export function loadChats() {
  try {
    const d = localStorage.getItem('messenger-chats')
    if (d) chats.set(JSON.parse(d))
  } catch {}
}
export function saveContacts() {
  try { localStorage.setItem('messenger-contacts', JSON.stringify(get(contacts))) } catch {}
}
export function loadContacts() {
  try {
    const d = localStorage.getItem('messenger-contacts')
    if (d) contacts.set(JSON.parse(d))
  } catch {}
}
export function saveProfile() {
  try { localStorage.setItem('messenger-profile', JSON.stringify(get(profile))) } catch {}
}
export function loadProfile() {
  try {
    const d = localStorage.getItem('messenger-profile')
    if (d) profile.set(JSON.parse(d))
  } catch {}
}
