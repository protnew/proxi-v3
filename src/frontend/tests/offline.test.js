import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import 'fake-indexeddb/auto';
import {
  init, saveMessage, getMessagesSince, getLastSync, setLastSync,
  savePending, getPending, clearPending, syncFromServer, flushPending, getMessageCount
} from '../src/lib/offline.js';

describe('offline storage', () => {
  beforeEach(async () => {
    // Clear indexedDB state
    const req = indexedDB.deleteDatabase('proxi-messenger');
    await new Promise((resolve) => {
      req.onsuccess = resolve;
      req.onerror = resolve;
    });
    // Need to reset internal db var somehow, but init() assigns it
    await init();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize and return db', async () => {
    const db = await init();
    expect(db.name).toBe('proxi-messenger');
    expect(db.version).toBe(1);
  });

  it('should save and retrieve a message', async () => {
    const msg = { from: 'Alice', text: 'Hello', ts: 1000 };
    await saveMessage(msg);
    const msgs = await getMessagesSince(500);
    expect(msgs.length).toBe(1);
    expect(msgs[0].from).toBe('Alice');
    expect(msgs[0].text).toBe('Hello');
  });

  it('should get message count', async () => {
    await saveMessage({ from: 'Bob', text: '1', ts: 1000 });
    await saveMessage({ from: 'Bob', text: '2', ts: 1001 });
    const count = await getMessageCount();
    expect(count).toBe(2);
  });

  it('should save and get last sync', async () => {
    let sync = await getLastSync();
    expect(sync).toBe(0);
    await setLastSync(123456);
    sync = await getLastSync();
    expect(sync).toBe(123456);
  });

  it('should save, get, and clear pending', async () => {
    await savePending({ type: 'msg', from: 'C', text: 'T', ts: 1 });
    let p = await getPending();
    expect(p.length).toBe(1);
    expect(p[0].text).toBe('T');
    
    await clearPending();
    p = await getPending();
    expect(p.length).toBe(0);
  });

  it('should sync from server successfully', async () => {
    window.myId = 'testuser';
    global.fetch = vi.fn().mockResolvedValue({
      json: vi.fn().mockResolvedValue({
        messages: [{ id: 'm1', from: 'S', text: 'Srv', ts: 2000 }]
      })
    });

    const added = await syncFromServer();
    expect(added).toBe(1);
    
    const count = await getMessageCount();
    expect(count).toBe(1);
    const sync = await getLastSync();
    expect(sync).toBe(2000);
  });

  it('should handle empty sync from server', async () => {
    window.myId = 'testuser';
    global.fetch = vi.fn().mockResolvedValue({
      json: vi.fn().mockResolvedValue({
        messages: []
      })
    });

    const added = await syncFromServer();
    expect(added).toBe(0);
  });

  it('should handle sync failure', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));
    const added = await syncFromServer();
    expect(added).toBe(-1);
  });

  it('should flush pending successfully', async () => {
    await savePending({ type: 'msg', from: 'A', text: 'Hello' });
    
    const ws = {
      readyState: WebSocket.OPEN || 1,
      send: vi.fn()
    };
    
    const sent = await flushPending(ws);
    expect(sent).toBe(1);
    expect(ws.send).toHaveBeenCalledOnce();
    
    const p = await getPending();
    expect(p.length).toBe(0);
  });

  it('should handle flush failure', async () => {
    await savePending({ type: 'msg', from: 'A', text: 'Hello' });
    
    const ws = {
      readyState: 1,
      send: vi.fn().mockImplementation(() => { throw new Error('ws error'); })
    };
    
    const sent = await flushPending(ws);
    expect(sent).toBe(0);
    const p = await getPending();
    expect(p.length).toBe(1); // not cleared
  });
});
