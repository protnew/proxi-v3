/**
 * MSG-102: Chat history IndexedDB persistence
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { saveMessage, loadMessages, clearHistory } from '../src/lib/chat-history'

// fake-indexeddb
import 'fake-indexeddb/auto'

describe('MSG-102 Chat History (IndexedDB)', () => {
  beforeEach(async () => {
    await clearHistory()
  })

  it('saves and loads messages by chatId', async () => {
    await saveMessage('dm:alice', {
      chatId: 'dm:alice', id: 'm1', from: 'alice', to: 'bob',
      text: 'Hello', timestamp: Date.now() - 5000, type: 'text', read: false,
    })
    await saveMessage('dm:alice', {
      chatId: 'dm:alice', id: 'm2', from: 'bob', to: 'alice',
      text: 'Hi!', timestamp: Date.now(), type: 'text', read: false,
    })
    await saveMessage('dm:carol', {
      chatId: 'dm:carol', id: 'm3', from: 'carol', to: 'bob',
      text: 'Other', timestamp: Date.now(), type: 'text', read: false,
    })

    const aliceMsgs = await loadMessages('dm:alice')
    expect(aliceMsgs.length).toBe(2)
    expect(aliceMsgs[0].text).toBe('Hello')
    expect(aliceMsgs[1].text).toBe('Hi!')

    const carolMsgs = await loadMessages('dm:carol')
    expect(carolMsgs.length).toBe(1)
  })

  it('clearHistory removes specific chat only', async () => {
    await saveMessage('dm:a', { chatId: 'dm:a', id: '1', from: 'a', to: 'b', text: 'x', timestamp: 1, type: 'text', read: false })
    await saveMessage('dm:b', { chatId: 'dm:b', id: '2', from: 'b', to: 'c', text: 'y', timestamp: 2, type: 'text', read: false })

    await clearHistory('dm:a')
    expect((await loadMessages('dm:a')).length).toBe(0)
    expect((await loadMessages('dm:b')).length).toBe(1)
  })

  it('messages sorted by timestamp ascending', async () => {
    await saveMessage('dm:x', { chatId: 'dm:x', id: 'late', from: 'a', to: 'b', text: 'later', timestamp: 2000, type: 'text', read: false })
    await saveMessage('dm:x', { chatId: 'dm:x', id: 'early', from: 'a', to: 'b', text: 'earlier', timestamp: 1000, type: 'text', read: false })

    const msgs = await loadMessages('dm:x')
    expect(msgs[0].id).toBe('early')
    expect(msgs[1].id).toBe('late')
  })
})
