import { describe, it, expect, beforeEach, vi } from 'vitest'

// Mock localStorage for node environment BEFORE importing api
const store: Record<string, string> = {}
const localStorageMock = {
  getItem: (key: string) => store[key] ?? null,
  setItem: (key: string, value: string) => { store[key] = value },
  removeItem: (key: string) => { delete store[key] },
  clear: () => { Object.keys(store).forEach(k => delete store[k]) },
  get length() { return Object.keys(store).length },
  key: (i: number) => Object.keys(store)[i] ?? null,
}
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })
Object.defineProperty(globalThis, 'window', {
  value: { localStorage: localStorageMock, location: { protocol: 'http:', host: 'localhost:5173' } },
  writable: true,
})

const mockFetch = vi.fn()
global.fetch = mockFetch as any

// Import production request() which builds Authorization from proxi_token
import { request } from '../src/lib/api'

describe('api.ts request() auth header (production code)', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    localStorage.clear()
  })

  it('does not send Authorization when proxi_token missing', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: 'unauthorized' }),
    })

    await request('/api/messages/send', {
      method: 'POST',
      body: JSON.stringify({ to: 'bob', text: 'x' }),
    })

    expect(mockFetch).toHaveBeenCalled()
    const init = mockFetch.mock.calls[0][1] || {}
    const headers = init.headers || {}
    const auth = headers['Authorization'] || headers['authorization']
    expect(auth).toBeFalsy()
  })

  it('sends Bearer token when proxi_token present', async () => {
    localStorage.setItem('proxi_token', 'fake.jwt.token')
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ ok: true }),
    })

    await request('/api/health', { method: 'GET' })

    const init = mockFetch.mock.calls[0][1] || {}
    const headers = init.headers || {}
    const auth = headers['Authorization'] || headers['authorization']
    expect(auth).toBe('Bearer fake.jwt.token')
  })

  it('returns error payload on 401 without throwing necessarily', async () => {
    localStorage.clear()
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: 'unauthorized' }),
    })
    const res = await request('/api/messages/send', { method: 'POST', body: '{}' })
    // request() wraps status — accept either error field or ok=false shape
    expect(res).toBeTruthy()
  })
})
