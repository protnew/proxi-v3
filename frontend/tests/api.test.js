import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import {
  getToken, setToken, clearToken, apiFetch, signup, login, getIdentity,
  getMessages, sendMessage, getContacts, addContact, getProfiles,
  getChannels, createChannel, getGroups, createGroup,
  setupSwitch, checkInSwitch, createWebSocket
} from '../src/lib/api.js';

describe('API Client', () => {
  let originalFetch;

  beforeEach(() => {
    // mock localStorage
    const store = {};
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key) => store[key] || null),
      setItem: vi.fn((key, value) => { store[key] = value.toString(); }),
      removeItem: vi.fn((key) => { delete store[key]; })
    });

    originalFetch = global.fetch;
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    global.fetch = originalFetch;
  });

  it('should manage tokens', () => {
    expect(getToken()).toBe('');
    setToken('test-token');
    expect(getToken()).toBe('test-token');
    clearToken();
    expect(getToken()).toBe('');
  });

  it('should handle token errors', () => {
    // If localStorage throws
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => { throw new Error('denied'); }),
      setItem: vi.fn(() => { throw new Error('denied'); }),
      removeItem: vi.fn(() => { throw new Error('denied'); })
    });
    
    expect(getToken()).toBe('');
    expect(() => setToken('t')).not.toThrow();
    expect(() => clearToken()).not.toThrow();
  });

  describe('apiFetch', () => {
    it('should fetch successfully', async () => {
      global.fetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ success: true })
      });
      const data = await apiFetch('/test');
      expect(data.success).toBe(true);
    });

    it('should handle 204 No Content', async () => {
      global.fetch.mockResolvedValueOnce({
        ok: true,
        status: 204
      });
      const data = await apiFetch('/test-204');
      expect(data).toBeNull();
    });

    it('should handle fetch errors', async () => {
      global.fetch.mockResolvedValueOnce({
        ok: false,
        statusText: 'Not Found',
        json: async () => ({ error: 'Custom Error' })
      });
      await expect(apiFetch('/fail')).rejects.toThrow('Custom Error');
    });

    it('should handle fetch network error', async () => {
      global.fetch.mockRejectedValueOnce(new Error('Network Fail'));
      await expect(apiFetch('/fail2')).rejects.toThrow('Network Fail');
    });
  });

  describe('Endpoints', () => {
    beforeEach(() => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({ token: 'new-token' })
      });
    });

    it('signup', async () => {
      const res = await signup('pass');
      expect(res.token).toBe('new-token');
      expect(getToken()).toBe('new-token');
    });

    it('login', async () => {
      const res = await login('npub1', 'pass');
      expect(res.token).toBe('new-token');
    });

    it('getIdentity', async () => {
      await getIdentity();
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/identity'), expect.anything());
    });

    it('getMessages', async () => {
      await getMessages(10);
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/messages?limit=10'), expect.anything());
    });

    it('sendMessage', async () => {
      await expect(sendMessage('to', '')).rejects.toThrow();
      await sendMessage('to', 'hello');
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/messages'), expect.objectContaining({ method: 'POST' }));
    });

    it('contacts & profiles', async () => {
      await getContacts();
      await addContact('npub2', 'Bob');
      await getProfiles(['npub1', 'npub2']);
      expect(global.fetch).toHaveBeenCalledTimes(3);
    });

    it('channels & groups', async () => {
      await getChannels();
      await createChannel('chan', 'desc');
      await getGroups();
      await createGroup('group', ['npub1']);
      expect(global.fetch).toHaveBeenCalledTimes(4);
    });

    it('advanced switch', async () => {
      await setupSwitch('npub2', 'msg', 7);
      await checkInSwitch();
      expect(global.fetch).toHaveBeenCalledTimes(2);
    });
  });

  describe('WebSocket helper', () => {
    it('should create WebSocket instance', () => {
      global.WebSocket = class { constructor(url) { this.url = url; } };
      const ws = createWebSocket('/test-ws');
      expect(ws.url).toContain('ws://');
      expect(ws.url).toContain('/test-ws');
    });
  });
});
