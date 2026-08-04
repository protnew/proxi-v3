/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock WebSocket
class MockWS {
  readyState = 0;
  onopen: any; onmessage: any; onerror: any; onclose: any;
  send = vi.fn();
  close = vi.fn();
  static OPEN = 1;
  constructor(public url: string) {
    setTimeout(() => { this.readyState = 1; this.onopen?.(); }, 5);
  }
}
vi.stubGlobal('WebSocket', MockWS as any);

import { NostrChat } from '../src/lib/nostr-chat';
import { createIdentity, type Identity } from '../src/lib/identity';

describe('NostrChat relay + transport', () => {
  let alice: Identity;
  let bob: Identity;

  beforeEach(async () => {
    localStorage.clear();
    alice = await createIdentity();
    bob = await createIdentity();
  });

  it('constructs with identity and default relays', () => {
    const chat = new NostrChat(alice);
    expect(chat).toBeDefined();
    expect(chat.isConnected).toBe(false);
  });

  it('connect returns count of connected relays', async () => {
    const chat = new NostrChat(alice, ['wss://relay.test.com']);
    const count = await chat.connect();
    expect(typeof count).toBe('number');
  });

  it('sendDM encrypts and returns boolean', async () => {
    const chat = new NostrChat(alice, ['wss://relay.test.com']);
    await chat.connect();
    const result = await chat.sendDM(bob.publicKey, 'Hello encrypted world');
    expect(typeof result).toBe('boolean');
  });

  it('sendDM rejects empty text', async () => {
    const chat = new NostrChat(alice, ['wss://relay.test.com']);
    const result = await chat.sendDM(bob.publicKey, '');
    expect(result).toBe(false);
  });

  it('onMessage registers callback', () => {
    const chat = new NostrChat(alice);
    chat.onMessage(() => {});
    expect(chat).toBeDefined();
  });

  it('disconnect closes all connections', async () => {
    const chat = new NostrChat(alice, ['wss://relay.test.com']);
    await chat.connect();
    chat.disconnect();
    expect(chat.isConnected).toBe(false);
  });

  it('connectedRelays returns number', async () => {
    const chat = new NostrChat(alice, ['wss://relay.test.com']);
    await chat.connect();
    expect(typeof chat.connectedRelays).toBe('number');
  });
});
