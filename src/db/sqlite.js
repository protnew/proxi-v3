export function initDB(db) {
  // Set WAL mode for performance and concurrency
  db.pragma('journal_mode = WAL');
  db.pragma('synchronous = NORMAL');
  
  // Create messages table
  db.exec(`
    CREATE TABLE IF NOT EXISTS messages (
      id TEXT PRIMARY KEY,
      content TEXT NOT NULL,
      timestamp INTEGER NOT NULL
    );
  `);
}

export function saveMessage(db, id, content, timestamp) {
  const stmt = db.prepare('INSERT INTO messages (id, content, timestamp) VALUES (?, ?, ?)');
  stmt.run(id, content, timestamp);
}

export function cleanupOldMessages(db, maxAgeMs) {
  const cutoff = Date.now() - maxAgeMs;
  const stmt = db.prepare('DELETE FROM messages WHERE timestamp < ?');
  stmt.run(cutoff);
  
  // Secure overwrite / WAL checkpoint could be forced here for PFS
  db.pragma('wal_checkpoint(TRUNCATE)');
}
