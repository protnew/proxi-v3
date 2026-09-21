/**
 * PWA-005: Nano Store — chat state
 */
import { atom, computed } from 'nanostores'

export interface ChatMessage {
  id: string
  from: string
  to: string
  text: string
  timestamp: number
  type: string
  read: boolean
}

export interface Chat {
  id: string
  peerPubkey: string
  peerName: string
  messages: ChatMessage[]
  unread: number
  typing: boolean
}

export const chatsStore = atom<Chat[]>([])
export const activeChatStore = atom<string | null>(null)

export function ensureDMChat(peerPubkey: string, peerName: string) {
  const chatId = `dm:${peerPubkey}`
  const chats = chatsStore.get()
  if (!chats.find(c => c.id === chatId)) {
    chatsStore.set([...chats, {
      id: chatId, peerPubkey, peerName,
      messages: [], unread: 0, typing: false,
    }])
  }
  return chatId
}

export function addMessage(chatId: string, msg: ChatMessage) {
  const chats = chatsStore.get()
  const chat = chats.find(c => c.id === chatId)
  if (chat) {
    chat.messages.push(msg)
    if (activeChatStore.get() !== chatId) chat.unread++
    chatsStore.set([...chats])
  }
}

export function loadChats(chats: Chat[]) {
  chatsStore.set(chats)
}

export function setTyping(chatId: string, pubkey: string) {
  const chats = chatsStore.get()
  const chat = chats.find(c => c.id === chatId)
  if (chat) {
    chat.typing = true
    chatsStore.set([...chats])
    setTimeout(() => {
      const c2 = chatsStore.get().find(c => c.id === chatId)
      if (c2) { c2.typing = false; chatsStore.set([...chatsStore.get()]) }
    }, 3000)
  }
}

export function markRead(chatId: string) {
  const chats = chatsStore.get()
  const chat = chats.find(c => c.id === chatId)
  if (chat) {
    chat.unread = 0
    chat.messages.forEach(m => m.read = true)
    chatsStore.set([...chats])
  }
}

export const totalUnread = computed(chatsStore, chats =>
  chats.reduce((sum, c) => sum + c.unread, 0)
)

export const sortedChats = computed(chatsStore, chats =>
  [...chats].sort((a, b) => {
    const aT = a.messages.length ? a.messages[a.messages.length - 1].timestamp : 0
    const bT = b.messages.length ? b.messages[b.messages.length - 1].timestamp : 0
    return bT - aT
  })
)
