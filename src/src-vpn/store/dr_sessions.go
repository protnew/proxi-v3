package store

import "time"

// P13 (2026-09-20): DR session markers. Ratchet state itself is not yet
// serialized to disk, but we persist that a session WAS established —
// so after a restart the sender fails closed (re-handshake required)
// instead of silently downgrading to plaintext / re-encrypting.
//
// TODO(P13-full): serialize crypto.RatchetState (sealed via identityKey)
// so sessions survive restarts without re-handshake.

// SaveDRSessionMarker records that a DR session exists for local→remote.
func (s *Store) SaveDRSessionMarker(localNpub, remoteNpub string) error {
	_, err := s.db.Exec(
		`CREATE TABLE IF NOT EXISTS dr_session_markers (
			local_npub TEXT NOT NULL,
			remote_npub TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (local_npub, remote_npub)
		)`)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = s.db.Exec(
		`INSERT INTO dr_session_markers (local_npub, remote_npub, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(local_npub, remote_npub) DO UPDATE SET updated_at=excluded.updated_at`,
		localNpub, remoteNpub, now, now)
	return err
}

// HasDRSessionMarker reports whether a DR session marker exists for local→remote.
func (s *Store) HasDRSessionMarker(localNpub, remoteNpub string) bool {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM dr_session_markers WHERE local_npub=? AND remote_npub=?`,
		localNpub, remoteNpub).Scan(&n)
	return err == nil && n > 0
}

// DeleteDRSessionMarker removes the marker (re-handshake / teardown).
func (s *Store) DeleteDRSessionMarker(localNpub, remoteNpub string) error {
	_, err := s.db.Exec(
		`DELETE FROM dr_session_markers WHERE local_npub=? AND remote_npub=?`,
		localNpub, remoteNpub)
	return err
}
