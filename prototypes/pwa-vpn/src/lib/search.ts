/**
 * Full-text search across all chats and messages
 */

import { get } from 'svelte/store'
import { chats, type Chat, type Message } from '../stores/messenger'

export interface SearchResult {
  chatId: string
  chatName: string
  message: Message
  matchIndex: number // position of match in text
}

/**
 * Search messages across all chats
 */
export function searchMessages(query: string, limit = 50): SearchResult[] {
  if (!query.trim()) return []
  const q = query.toLowerCase()
  const allChats = get(chats)
  const results: SearchResult[] = []

  for (const chat of allChats) {
    for (const msg of chat.messages) {
      const idx = msg.text.toLowerCase().indexOf(q)
      if (idx >= 0) {
        results.push({
          chatId: chat.id,
          chatName: chat.name,
          message: msg,
          matchIndex: idx,
        })
        if (results.length >= limit) return results
      }
    }
  }

  // Sort by timestamp descending
  return results.sort((a, b) => b.message.timestamp - a.message.timestamp)
}

/**
 * Search within a single chat
 */
export function searchInChat(chatId: string, query: string): Message[] {
  if (!query.trim()) return []
  const q = query.toLowerCase()
  const allChats = get(chats)
  const chat = allChats.find(c => c.id === chatId)
  if (!chat) return []
  return chat.messages.filter(m => m.text.toLowerCase().includes(q))
}
