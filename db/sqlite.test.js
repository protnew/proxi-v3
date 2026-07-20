import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { initDB, saveMessage, cleanupOldMessages } from './sqlite.js';
import fs from 'fs';
import path from 'path';

describe('SQLite WAL Architecture & PFS', () => {
  const dbPath = path.join(__dirname, 'test.db');
  let db;

  beforeEach(() => {
    if (fs.existsSync(dbPath)) fs.unlinkSync(dbPath);
    if (fs.existsSync(dbPath + '-wal')) fs.unlinkSync(dbPath + '-wal');
    db = new Database(dbPath);
    initDB(db);
  });

  afterEach(() => {
    db.close();
    if (fs.existsSync(dbPath)) fs.unlinkSync(dbPath);
    if (fs.existsSync(dbPath + '-wal')) fs.unlinkSync(dbPath + '-wal');
  });

  it('should initialize DB with WAL mode', () => {
    const mode = db.pragma('journal_mode', { simple: true });
    expect(mode.toLowerCase()).toBe('wal');
  });

  it('should save and physical auto-delete messages (PFS test)', () => {
    // Save a message
    saveMessage(db, 'msg1', 'Hello', Date.now() - 100000); // Old message
    saveMessage(db, 'msg2', 'World', Date.now()); // New message
    
    let count = db.prepare('SELECT count(*) as c FROM messages').get().c;
    expect(count).toBe(2);

    // Run cleanup (delete messages older than 50 seconds)
    cleanupOldMessages(db, 50000);

    count = db.prepare('SELECT count(*) as c FROM messages').get().c;
    expect(count).toBe(1);

    const msg = db.prepare('SELECT * FROM messages').get();
    expect(msg.id).toBe('msg2');
  });
});
