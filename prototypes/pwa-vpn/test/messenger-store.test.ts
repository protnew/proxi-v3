import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  chatsStore, activeChatStore, ensureDMChat, addMessage, loadChats, markRead, totalUnread, sortedChats,
} from '../src/stores/chats'
import { profileStore, setProfile, clearProfile } from '../src/stores/profile'
import {
  chats, contacts, profile, activeChatId, ensureDMChat as mEnsure, ensureGroupChat,
  addMessage as mAdd, markRead as mRead, editMessage, deleteMessage, addReaction,
  addContact, setContactOnline, totalUnread as mUnread, saveChats, loadChats as mLoad,
  saveContacts, loadContacts, saveProfile, loadProfile,
} from '../src/stores/messenger'
import { get } from 'svelte/store'

describe('stores/chats', () => {
  beforeEach(() => {
    chatsStore.set([])
    activeChatStore.set(null)
  })
  it('ensureDM + addMessage unread when not active', () => {
    const id = ensureDMChat('pk1', 'Alice')
    expect(id).toBe('dm:pk1')
    addMessage(id, { id: 'm1', from: 'pk1', to: 'me', text: 'hi', timestamp: 1, type: 'text', read: false })
    expect(chatsStore.get()[0].unread).toBe(1)
    expect(totalUnread.get()).toBe(1)
    markRead(id)
    expect(totalUnread.get()).toBe(0)
  })
  it('sortedChats newest first', () => {
    ensureDMChat('a', 'A')
    ensureDMChat('b', 'B')
    addMessage('dm:a', { id: '1', from: 'a', to: 'me', text: 'old', timestamp: 10, type: 't', read: true })
    addMessage('dm:b', { id: '2', from: 'b', to: 'me', text: 'new', timestamp: 99, type: 't', read: true })
    expect(sortedChats.get()[0].peerPubkey).toBe('b')
  })
  it('loadChats replaces', () => {
    loadChats([{ id: 'x', peerPubkey: 'p', peerName: 'P', messages: [], unread: 0, typing: false }])
    expect(chatsStore.get()).toHaveLength(1)
  })
})

describe('stores/profile', () => {
  it('set and clear', () => {
    setProfile({ name: 'Alex', npub: 'npub1x' })
    expect(profileStore.get().name).toBe('Alex')
    clearProfile()
    expect(profileStore.get().npub).toBe('')
    expect(profileStore.get().name).toBe('User')
  })
})

describe('stores/messenger', () => {
  beforeEach(() => {
    chats.set([])
    contacts.set([])
    profile.set({ pubkey: '', name: 'User', about: '', avatar: '👤' })
    activeChatId.set(null)
    localStorage.clear()
  })
  it('DM + group + message lifecycle', () => {
    const dm = mEnsure('pk2', 'Bob')
    const g = ensureGroupChat('g1', 'Team', ['pk2'])
    mAdd(dm, { id: 'm1', from: 'pk2', to: 'me', text: 'hello', timestamp: Date.now(), type: 'text', read: false })
    expect(get(mUnread)).toBe(1)
    mRead(dm)
    expect(get(mUnread)).toBe(0)
    editMessage(dm, 'm1', 'hello!')
    let msg = get(chats).find(c => c.id === dm)!.messages[0]
    expect(msg.text).toBe('hello!')
    expect(msg.edited).toBe(true)
    addReaction(dm, 'm1', '👍', 'me')
    msg = get(chats).find(c => c.id === dm)!.messages[0]
    expect(msg.reactions?.['👍']).toEqual(['me'])
    deleteMessage(dm, 'm1')
    expect(get(chats).find(c => c.id === dm)!.messages).toHaveLength(0)
    expect(g).toBe('group:g1')
  })
  it('contacts persist', () => {
    addContact('pk9', 'Nina')
    setContactOnline('pk9', true)
    saveContacts()
    contacts.set([])
    loadContacts()
    expect(get(contacts)[0].name).toBe('Nina')
    expect(get(contacts)[0].isOnline).toBe(true)
  })
  it('profile persist', () => {
    profile.set({ pubkey: 'aa', name: 'Me', about: '', avatar: '' })
    saveProfile()
    profile.set({ pubkey: '', name: 'User', about: '', avatar: '👤' })
    loadProfile()
    expect(get(profile).name).toBe('Me')
  })
  it('chats persist', () => {
    mEnsure('pk3', 'Cara')
    saveChats()
    chats.set([])
    mLoad()
    expect(get(chats).some(c => c.name === 'Cara' || c.id.includes('pk3'))).toBe(true)
  })
})
