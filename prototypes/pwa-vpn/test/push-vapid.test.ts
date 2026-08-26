/**
 * PUSH-001/002/003: WebPush VAPID + notification flow tests
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock browser APIs
globalThis.PushManager = class {} as any
;(globalThis as any).atob = (s: string) => Buffer.from(s, 'base64').toString('binary')
;(globalThis as any).btoa = (s: string) => Buffer.from(s, 'binary').toString('base64')

const mockShowNotification = vi.fn()
const mockSub = {
  toJSON: () => ({ endpoint: 'https://push.example.com/123', keys: { p256dh: 'abc', auth: 'def' } }),
  unsubscribe: vi.fn().mockResolvedValue(true),
}
const mockPushManager = {
  getSubscription: vi.fn().mockResolvedValue(null),
  subscribe: vi.fn().mockResolvedValue(mockSub),
}
const mockSWReg = { pushManager: mockPushManager, showNotification: mockShowNotification }

Object.defineProperty(globalThis, 'navigator', {
  value: {
    serviceWorker: {
      register: vi.fn().mockResolvedValue(mockSWReg),
      ready: Promise.resolve(mockSWReg),
    },
  },
  writable: true,
  configurable: true,
})

let _perm = 'granted'
Object.defineProperty(globalThis, 'Notification', {
  value: class {
    static get permission() { return _perm }
    static requestPermission = vi.fn().mockImplementation(() => Promise.resolve(_perm))
  },
  writable: true,
  configurable: true,
})

const store: Record<string, string> = {}
Object.defineProperty(globalThis, 'localStorage', {
  value: {
    getItem: vi.fn((k: string) => store[k] || null),
    setItem: vi.fn((k: string, v: string) => { store[k] = v }),
    removeItem: vi.fn((k: string) => { delete store[k] }),
    clear: vi.fn(),
  },
  writable: true,
  configurable: true,
})

import { subscribePush, unsubscribePush, hasPushSubscription, getPushPermission, notifyLocally } from '../src/lib/push'

describe('PUSH-001 WebPush VAPID', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.keys(store).forEach(k => delete store[k])
    mockPushManager.getSubscription.mockResolvedValue(null)
    mockPushManager.subscribe.mockResolvedValue(mockSub)
  })

  it('subscribePush creates subscription and stores in localStorage', async () => {
    const sub = await subscribePush()
    expect(sub).not.toBeNull()
    expect(mockPushManager.subscribe).toHaveBeenCalled()
    expect(store['proxi-push-sub']).toBeDefined()
  })

  it('hasPushSubscription reflects localStorage', async () => {
    expect(hasPushSubscription()).toBe(false)
    await subscribePush()
    expect(hasPushSubscription()).toBe(true)
  })

  it('unsubscribePush removes subscription', async () => {
    await subscribePush()
    expect(hasPushSubscription()).toBe(true)
    await unsubscribePush()
    expect(hasPushSubscription()).toBe(false)
  })
})

describe('PUSH-003 permission', () => {
  it('getPushPermission returns true when granted', async () => {
    const ok = await getPushPermission()
    expect(ok).toBe(true)
  })
})

describe('PUSH-002 local notification', () => {
  it('notifyLocally fires Notification when permission granted', async () => {
    let ncalled = 0
    const ONC = (globalThis as any).Notification
    ;(globalThis as any).Notification = class {
      static get permission() { return 'granted' }
      constructor() { ncalled++ }
    }
    try {
      await notifyLocally('Test', 'Body')
      expect(ncalled).toBeGreaterThan(0)
    } finally {
      ;(globalThis as any).Notification = ONC
    }
  })
})
