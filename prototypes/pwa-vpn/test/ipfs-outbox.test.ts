import { describe, it, expect, beforeEach } from 'vitest'
import { computeCID, formatCIDShort, MAX_FILE_BYTES } from '../src/lib/ipfs-storage'
import { enqueue, listOutbox, remove, clearOutbox, flushOutbox } from '../src/lib/offline-outbox'

describe('ipfs-storage', () => {
  it('computeCID is stable sha256', async () => {
    const a = await computeCID(new TextEncoder().encode('hello indestructible'))
    const b = await computeCID(new TextEncoder().encode('hello indestructible'))
    const c = await computeCID(new TextEncoder().encode('different'))
    expect(a).toBe(b)
    expect(a.startsWith('sha256:')).toBe(true)
    expect(a).not.toBe(c)
    expect(a.length).toBe(7 + 64)
  })

  it('formatCIDShort shortens long cids', () => {
    const cid = 'sha256:' + 'ab'.repeat(32)
    expect(formatCIDShort(cid).includes('…')).toBe(true)
    expect(formatCIDShort('short')).toBe('short')
  })

  it('MAX_FILE_BYTES is 25MB', () => {
    expect(MAX_FILE_BYTES).toBe(25 * 1024 * 1024)
  })
})

describe('offline-outbox', () => {
  beforeEach(() => {
    clearOutbox()
  })

  it('enqueue and list', () => {
    const item = enqueue('peer1', 'hello')
    expect(item.id).toBeTruthy()
    const list = listOutbox()
    expect(list.length).toBe(1)
    expect(list[0].text).toBe('hello')
  })

  it('flush removes successful items', async () => {
    enqueue('p1', 'a')
    enqueue('p2', 'b')
    const r = await flushOutbox(async (to, text) => ({ ok: true, via: 'test' }))
    expect(r.sent).toBe(2)
    expect(r.left).toBe(0)
    expect(listOutbox().length).toBe(0)
  })

  it('flush keeps failed items and bumps attempts', async () => {
    enqueue('p1', 'x')
    const r = await flushOutbox(async () => ({ ok: false, error: 'down' }))
    expect(r.sent).toBe(0)
    expect(r.left).toBe(1)
    expect(listOutbox()[0].attempts).toBe(1)
  })

  it('remove deletes by id', () => {
    const item = enqueue('p', 'z')
    remove(item.id)
    expect(listOutbox().length).toBe(0)
  })
})
