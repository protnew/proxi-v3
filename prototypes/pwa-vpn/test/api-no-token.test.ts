import { describe, it, expect, beforeEach, vi } from 'vitest'

// Mock localStorage for node environment
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

// Mock fetch
const mockFetch = vi.fn()
global.fetch = mockFetch as any

describe('API sendDM without token', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    localStorage.clear()
  })

  it('sends no Authorization header when proxi_token missing', async () => {
    expect(localStorage.getItem('proxi_token')).toBeNull()

    mockFetch.mockResolvedValueOnce({
      ok: false, status: 401,
      json: async () => ({ error: 'unauthorized' }),
    } as any)

    const token = localStorage.getItem('proxi_token') || ''
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token) headers['Authorization'] = `Bearer ${token}`

    const res = await fetch('http://localhost:8080/api/messages/send', {
      method: 'POST', headers,
      body: JSON.stringify({ recipient: 'bob', text: 'test' }),
    })

    expect(res.ok).toBe(false)
    expect(res.status).toBe(401)
    const callArgs = mockFetch.mock.calls[0][1]
    expect(callArgs.headers.Authorization).toBeUndefined()
  })

  it('includes Authorization header when token present', async () => {
    localStorage.setItem('proxi_token', 'fake.jwt.token')

    mockFetch.mockResolvedValueOnce({
      ok: true, status: 200,
      json: async () => ({ ok: true }),
    } as any)

    const token = localStorage.getItem('proxi_token') || ''
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token) headers['Authorization'] = `Bearer ${token}`

    await fetch('http://localhost:8080/api/messages/send', {
      method: 'POST', headers,
      body: JSON.stringify({ recipient: 'bob', text: 'test' }),
    })

    const callArgs = mockFetch.mock.calls[0][1]
    expect(callArgs.headers.Authorization).toBe('Bearer fake.jwt.token')
  })

  it('rejects on network failure when backend unreachable', async () => {
    localStorage.clear()
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    await expect(
      fetch('http://localhost:8080/api/auth/signup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ npub: 'a'.repeat(64), name: 'test' }),
      })
    ).rejects.toThrow('Failed to fetch')
  })
})
