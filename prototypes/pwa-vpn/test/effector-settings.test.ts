/**
 * @vitest-environment jsdom
 * Tests for settings.ts, effector.ts, and nostr-signaling.ts
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock WebSocket
class MockWS {
  readyState = 0;
  onopen: any; onmessage: any; onerror: any; onclose: any;
  send = vi.fn(); close = vi.fn();
  static OPEN = 1;
  constructor(public url: string) {
    setTimeout(() => { this.readyState = 1; this.onopen?.(); }, 5);
  }
}
vi.stubGlobal('WebSocket', MockWS as any);

// Mock localStorage
const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: vi.fn((k: string) => store[k] ?? null),
  setItem: vi.fn((k: string, v: string) => { store[k] = v; }),
  removeItem: vi.fn((k: string) => { delete store[k]; }),
  clear: vi.fn(() => { Object.keys(store).forEach(k => delete store[k]); }),
});

// Use Node's crypto for real SHA-256 (needed for Nostr event signing)
import { webcrypto } from 'crypto';
vi.stubGlobal('crypto', webcrypto);

import { $settings, updateSetting, resetSettings } from '../src/lib/settings';
import {
  $isAuthenticated, $currentUser, authenticate, logout,
  $isConnected, $vpnEnabled, setConnected, toggleVpn,
  $messages, $activeChat, messageReceived, setActiveChat,
} from '../src/lib/effector';
import { NostrSignaling, KIND_VPN_REQUEST, KIND_VPN_OFFER } from '../src/lib/nostr-signaling';
import { createIdentity } from '../src/lib/identity';

describe('Settings store', () => {
  it('has default values', () => {
    let s: any;
    const unsub = $settings.subscribe(v => s = v);
    expect(s.theme).toBe('light');
    expect(s.notifications).toBe(true);
    expect(s.soundEnabled).toBe(true);
    expect(s.vpnAutoConnect).toBe(false);
    expect(s.fontSize).toBe(14);
    expect(s.language).toBe('ru');
    unsub();
  });

  it('updateSetting changes a single key', () => {
    let s: any;
    const unsub = $settings.subscribe(v => s = v);
    updateSetting({ key: 'theme', value: 'dark' });
    expect(s.theme).toBe('dark');
    updateSetting({ key: 'fontSize', value: 18 });
    expect(s.fontSize).toBe(18);
    unsub();
  });

  it('resetSettings restores defaults', () => {
    let s: any;
    const unsub = $settings.subscribe(v => s = v);
    updateSetting({ key: 'theme', value: 'dark' });
    updateSetting({ key: 'fontSize', value: 20 });
    resetSettings();
    expect(s.theme).toBe('light');
    expect(s.fontSize).toBe(14);
    unsub();
  });
});

describe('Effector stores', () => {
  it('$isAuthenticated starts false', () => {
    let v: boolean = true;
    const unsub = $isAuthenticated.subscribe(val => v = val);
    expect(v).toBe(false);
    unsub();
  });

  it('authenticate sets auth state', () => {
    let authed = false;
    let user: any = null;
    const u1 = $isAuthenticated.subscribe(v => authed = v);
    const u2 = $currentUser.subscribe(v => user = v);
    authenticate({ pubkey: 'test-pub', name: 'Alice' });
    expect(authed).toBe(true);
    expect(user).toEqual({ pubkey: 'test-pub', name: 'Alice' });
    u1(); u2();
  });

  it('logout resets auth', () => {
    let authed = true;
    let user: any = { pubkey: 'x' };
    const u1 = $isAuthenticated.subscribe(v => authed = v);
    const u2 = $currentUser.subscribe(v => user = v);
    authenticate({ pubkey: 'test' });
    logout();
    expect(authed).toBe(false);
    expect(user).toBeNull();
    u1(); u2();
  });

  it('$isConnected updates via setConnected', () => {
    let v = false;
    const unsub = $isConnected.subscribe(val => v = val);
    setConnected(true);
    expect(v).toBe(true);
    setConnected(false);
    expect(v).toBe(false);
    unsub();
  });

  it('toggleVpn toggles $vpnEnabled', () => {
    let v = false;
    const unsub = $vpnEnabled.subscribe(val => v = val);
    toggleVpn();
    expect(v).toBe(true);
    toggleVpn();
    expect(v).toBe(false);
    unsub();
  });

  it('$messages accumulates via messageReceived', () => {
    let msgs: any;
    const unsub = $messages.subscribe(v => msgs = v);
    messageReceived({ chatId: 'c1', message: { text: 'hello' } });
    expect(msgs.c1).toBeDefined();
    expect(msgs.c1.length).toBe(1);
    messageReceived({ chatId: 'c1', message: { text: 'world' } });
    expect(msgs.c1.length).toBe(2);
    unsub();
  });

  it('$activeChat updates via setActiveChat', () => {
    let chat: any;
    const unsub = $activeChat.subscribe(v => chat = v);
    setActiveChat('chat-123');
    expect(chat).toBe('chat-123');
    setActiveChat(null);
    expect(chat).toBeNull();
    unsub();
  });
});

describe('NostrSignaling', () => {
  let identity: any;

  beforeEach(async () => {
    identity = await createIdentity();
  });

  it('constructs with identity and default relays', () => {
    const ns = new NostrSignaling(identity);
    expect(ns).toBeDefined();
  });

  it('constructs with custom relays', () => {
    const ns = new NostrSignaling(identity, ['wss://custom.relay.com']);
    expect(ns).toBeDefined();
  });

  it('start() connects to relays and subscribes', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    // Should not throw — WebSocket mock auto-connects
  });

  it('stop() closes all connections', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    ns.stop();
    // No error expected
  });

  it('sendRequest publishes VPN request event', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    // Should not throw
    await ns.sendRequest('target-pub-123');
  });

  it('sendOffer publishes offer', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    await ns.sendOffer('target-pub', 'mock-sdp');
  });

  it('sendAnswer publishes answer', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    await ns.sendAnswer('target-pub', 'mock-sdp');
  });

  it('sendIceCandidate publishes ICE candidate', async () => {
    const ns = new NostrSignaling(identity, ['wss://test.relay.com']);
    await ns.start(() => {});
    await ns.sendIceCandidate('target-pub', { candidate: 'ice-candidate' });
  });

  it('VPN constants are correct', () => {
    expect(KIND_VPN_REQUEST).toBe(30090);
    expect(KIND_VPN_OFFER).toBe(30091);
  });
});
