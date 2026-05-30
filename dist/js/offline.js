// Offline Storage module for Proxi Messenger
// Uses IndexedDB for message caching + sync on reconnect

const OfflineStorage = (() => {
  const DB_NAME = 'proxi-messenger';
  const DB_VERSION = 1;
  let db = null;

  // Initialize IndexedDB
  async function init() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      
      req.onupgradeneeded = (e) => {
        const db = e.target.result;
        
        // Messages store
        if (!db.objectStoreNames.contains('messages')) {
          const msgStore = db.createObjectStore('messages', { keyPath: 'id' });
          msgStore.createIndex('timestamp', 'timestamp', { unique: false });
          msgStore.createIndex('from', 'from', { unique: false });
          msgStore.createIndex('to', 'to', { unique: false });
        }
        
        // Pending messages (unsent while offline)
        if (!db.objectStoreNames.contains('pending')) {
          db.createObjectStore('pending', { keyPath: 'id', autoIncrement: true });
        }
        
        // Last sync timestamp
        if (!db.objectStoreNames.contains('meta')) {
          db.createObjectStore('meta', { keyPath: 'key' });
        }
      };

      req.onsuccess = (e) => {
        db = e.target.result;
        resolve(db);
      };

      req.onerror = (e) => reject(e.target.error);
    });
  }

  // Save message to local cache
  async function saveMessage(msg) {
    if (!db) return;
    return new Promise((resolve, reject) => {
      const tx = db.transaction('messages', 'readwrite');
      tx.objectStore('messages').put({
        id: msg.id || ('local-' + Date.now() + '-' + Math.random().toString(36).substr(2,6)),
        from: msg.from,
        to: msg.to || 'broadcast',
        text: msg.text,
        timestamp: msg.ts || Date.now()/1000|0,
        replyTo: msg.replyTo || null,
        forwardedFrom: msg.forwardedFrom || null,
        ttl: msg.ttl || 0,
        storedAt: Date.now(),
      });
      tx.oncomplete = () => resolve();
      tx.onerror = (e) => reject(e.target.error);
    });
  }

  // Get messages since timestamp
  async function getMessagesSince(sinceTs) {
    if (!db) return [];
    return new Promise((resolve, reject) => {
      const tx = db.transaction('messages', 'readonly');
      const store = tx.objectStore('messages');
      const index = store.index('timestamp');
      const range = IDBKeyRange.lowerBound(sinceTs, true);
      const req = index.getAll(range);
      req.onsuccess = () => resolve(req.result || []);
      req.onerror = (e) => reject(e.target.error);
    });
  }

  // Get last sync timestamp
  async function getLastSync() {
    if (!db) return 0;
    return new Promise((resolve) => {
      const tx = db.transaction('meta', 'readonly');
      const req = tx.objectStore('meta').get('lastSync');
      req.onsuccess = () => resolve(req.result?.value || 0);
      req.onerror = () => resolve(0);
    });
  }

  // Update last sync timestamp
  async function setLastSync(ts) {
    if (!db) return;
    return new Promise((resolve) => {
      const tx = db.transaction('meta', 'readwrite');
      tx.objectStore('meta').put({ key: 'lastSync', value: ts });
      tx.oncomplete = () => resolve();
    });
  }

  // Save pending message (to send when online)
  async function savePending(msg) {
    if (!db) return;
    return new Promise((resolve, reject) => {
      const tx = db.transaction('pending', 'readwrite');
      tx.objectStore('pending').put({
        type: msg.type,
        from: msg.from,
        to: msg.to,
        text: msg.text,
        ts: msg.ts,
        ttl: msg.ttl || 0,
        createdAt: Date.now(),
      });
      tx.oncomplete = () => resolve();
      tx.onerror = (e) => reject(e.target.error);
    });
  }

  // Get all pending messages
  async function getPending() {
    if (!db) return [];
    return new Promise((resolve, reject) => {
      const tx = db.transaction('pending', 'readonly');
      const req = tx.objectStore('pending').getAll();
      req.onsuccess = () => resolve(req.result || []);
      req.onerror = (e) => reject(e.target.error);
    });
  }

  // Clear pending messages after sending
  async function clearPending() {
    if (!db) return;
    return new Promise((resolve) => {
      const tx = db.transaction('pending', 'readwrite');
      tx.objectStore('pending').clear();
      tx.oncomplete = () => resolve();
    });
  }

  // Sync: fetch missed messages from server
  async function syncFromServer() {
    const lastSync = await getLastSync();
    try {
      const npub = window.myId || 'anonymous';
      const r = await fetch('/api/messages?since=' + lastSync + '&limit=200&npub=' + encodeURIComponent(npub));
      const d = await r.json();
      if (d.messages && d.messages.length > 0) {
        for (const msg of d.messages) {
          await saveMessage(msg);
        }
        // Update last sync to newest message timestamp
        const newestTs = Math.max(...d.messages.map(m => m.timestamp || m.ts || 0));
        await setLastSync(Math.max(newestTs, lastSync));
        return d.messages.length;
      }
      await setLastSync(Math.floor(Date.now()/1000));
      return 0;
    } catch(e) {
      console.warn('Sync failed:', e);
      return -1;
    }
  }

  // Flush pending messages to server
  async function flushPending(ws) {
    const pending = await getPending();
    if (pending.length === 0 || !ws || ws.readyState !== WebSocket.OPEN) return 0;
    
    let sent = 0;
    for (const msg of pending) {
      try {
        ws.send(JSON.stringify(msg));
        sent++;
      } catch(e) { break; }
    }
    if (sent > 0) await clearPending();
    return sent;
  }

  // Get message count
  async function getMessageCount() {
    if (!db) return 0;
    return new Promise((resolve) => {
      const tx = db.transaction('messages', 'readonly');
      const req = tx.objectStore('messages').count();
      req.onsuccess = () => resolve(req.result);
    });
  }

  return { 
    init, saveMessage, getMessagesSince, getLastSync, setLastSync,
    savePending, getPending, clearPending, syncFromServer, flushPending,
    getMessageCount 
  };
})();
