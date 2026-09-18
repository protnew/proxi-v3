import 'fake-indexeddb/auto'
import { describe, it, expect, beforeEach } from 'vitest'
import { getEmptyState } from '../src/lib/empty-states'
import { makeStatus, nextRetrySeconds } from '../src/lib/connection-status'
import { toast, getErrorLog, exportErrorLog, clearErrorLog, onToast } from '../src/lib/toast'
import { t, setLocale, getLocale } from '../src/lib/i18n'
import { importIdentity, createIdentity, deleteIdentity, wipeSecureStore } from '../src/lib/identity'

describe('ONB-003 empty states', () => {
  it('returns RU contacts empty', () => {
    const s = getEmptyState('contacts', 'ru')
    expect(s.title).toContain('Нет')
    expect(s.cta).toBeTruthy()
  })
  it('returns EN vpn_off', () => {
    const s = getEmptyState('vpn_off', 'en')
    expect(s.title.toLowerCase()).toContain('vpn')
  })
})

describe('CON-001 connection status', () => {
  it('labels searching_path', () => {
    const s = makeStatus('searching_path')
    expect(s.labelRu).toContain('путь')
    expect(s.phase).toBe('searching_path')
  })
  it('retry backoff caps', () => {
    expect(nextRetrySeconds(0)).toBe(2)
    expect(nextRetrySeconds(10)).toBe(30)
  })
})

describe('INF-001 toast + error log', () => {
  beforeEach(() => clearErrorLog())
  it('emits toast and logs errors', () => {
    const seen: string[] = []
    const off = onToast(t => seen.push(t.message))
    toast('hello', 'info')
    toast('boom', 'error')
    off()
    expect(seen).toContain('hello')
    expect(getErrorLog().some(e => e.message === 'boom')).toBe(true)
    expect(exportErrorLog()).toContain('boom')
  })
})

describe('INF-002 i18n', () => {
  it('switches RU/EN', () => {
    setLocale('ru')
    expect(t('vpn.connect')).toContain('VPN')
    setLocale('en')
    expect(getLocale()).toBe('en')
    expect(t('vpn.connect')).toBe('Connect VPN')
    setLocale('ru')
  })
})

describe('ONB-000 identity import/create', () => {
  beforeEach(async () => {
    localStorage.clear()
    await wipeSecureStore()
  })
  it('createIdentity yields npub/nsec', async () => {
    const id = await createIdentity()
    expect(id.npub.startsWith('npub1')).toBe(true)
    expect(id.nsec.startsWith('nsec1')).toBe(true)
    expect(localStorage.getItem('indestructible-seckey')).toBeNull()
  })
  it('importIdentity roundtrip nsec', async () => {
    const a = await createIdentity()
    await deleteIdentity()
    const b = await importIdentity(a.nsec)
    expect(b.npub).toBe(a.npub)
    expect(b.publicKey).toBe(a.publicKey)
    expect(localStorage.getItem('indestructible-seckey')).toBeNull()
  })
})
