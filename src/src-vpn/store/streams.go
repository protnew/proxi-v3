package store

import (
	"fmt"

	_ "modernc.org/sqlite"
)

func (s *Store) SaveProfile(npub, displayName, avatarURL, bio string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO user_profiles (npub, display_name, avatar_url, bio, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		npub, displayName, avatarURL, bio, nowUnix(),
	)
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

// GetProfile returns a user's profile.

func (s *Store) GetProfile(npub string) (map[string]interface{}, error) {
	var displayName, avatarURL, bio string
	var updatedAt int64
	err := s.db.QueryRow(
		`SELECT display_name, avatar_url, bio, updated_at FROM user_profiles WHERE npub = ?`,
		npub,
	).Scan(&displayName, &avatarURL, &bio, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return map[string]interface{}{
		"npub":        npub,
		"displayName": displayName,
		"avatarUrl":   avatarURL,
		"bio":         bio,
		"updatedAt":   updatedAt,
	}, nil
}

// SearchProfiles searches user profiles by display name or npub.

func (s *Store) SearchProfiles(query string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT npub, display_name, avatar_url, bio, updated_at
		 FROM user_profiles
		 WHERE display_name LIKE ? OR npub LIKE ?
		 ORDER BY updated_at DESC
		 LIMIT 50`,
		"%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}
	defer rows.Close()

	var profiles []map[string]interface{}
	for rows.Next() {
		var npub, displayName, avatarURL, bio string
		var updatedAt int64
		if err := rows.Scan(&npub, &displayName, &avatarURL, &bio, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, map[string]interface{}{
			"npub":        npub,
			"displayName": displayName,
			"avatarUrl":   avatarURL,
			"bio":         bio,
			"updatedAt":   updatedAt,
		})
	}
	return profiles, rows.Err()
}

// ---------------------------------------------------------------------------
// Search Messages
// ---------------------------------------------------------------------------

// SearchMessages performs a text search across messages visible to the given npub.

type FileMeta struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Type       string `json:"type"`
	UploadedBy string `json:"uploadedBy"`
	CreatedAt  int64  `json:"createdAt"`
}

// SaveFileMeta persists file metadata.

func (s *Store) SaveFileMeta(fm FileMeta) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO file_metadata (id, name, size, type, uploaded_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		fm.ID, fm.Name, fm.Size, fm.Type, fm.UploadedBy, fm.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save file meta: %w", err)
	}
	return nil
}

// GetFileMeta returns metadata for a single file.

func (s *Store) GetFileMeta(id string) (*FileMeta, error) {
	var fm FileMeta
	err := s.db.QueryRow(
		`SELECT id, name, size, type, uploaded_by, created_at FROM file_metadata WHERE id = ?`,
		id,
	).Scan(&fm.ID, &fm.Name, &fm.Size, &fm.Type, &fm.UploadedBy, &fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get file meta: %w", err)
	}
	return &fm, nil
}

// ListFileMeta returns metadata for all uploaded files.

func (s *Store) ListFileMeta() ([]FileMeta, error) {
	rows, err := s.db.Query(
		`SELECT id, name, size, type, uploaded_by, created_at FROM file_metadata ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list file meta: %w", err)
	}
	defer rows.Close()

	var files []FileMeta
	for rows.Next() {
		var fm FileMeta
		if err := rows.Scan(&fm.ID, &fm.Name, &fm.Size, &fm.Type, &fm.UploadedBy, &fm.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan file meta: %w", err)
		}
		files = append(files, fm)
	}
	return files, rows.Err()
}

// DeleteFileMeta removes file metadata by ID.

func (s *Store) DeleteFileMeta(id string) error {
	res, err := s.db.Exec(`DELETE FROM file_metadata WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file meta %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("file %s not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// time helper
// ---------------------------------------------------------------------------

type Stream struct {
	ID        string `json:"id"`
	Creator   string `json:"creator"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

func (s *Store) SaveStream(st Stream) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO streams (id, creator, title, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		st.ID, st.Creator, st.Title, st.Status, st.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save stream: %w", err)
	}
	return nil
}

func (s *Store) GetStream(id string) (*Stream, error) {
	var st Stream
	err := s.db.QueryRow(
		`SELECT id, creator, title, status, created_at FROM streams WHERE id = ?`,
		id,
	).Scan(&st.ID, &st.Creator, &st.Title, &st.Status, &st.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	return &st, nil
}

func (s *Store) UpdateStreamStatus(id, status string) error {
	res, err := s.db.Exec(`UPDATE streams SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update stream status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("stream %s not found", id)
	}
	return nil
}

func (s *Store) ListActiveStreams() ([]Stream, error) {
	rows, err := s.db.Query(
		`SELECT id, creator, title, status, created_at FROM streams WHERE status = 'active' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active streams: %w", err)
	}
	defer rows.Close()

	var streams []Stream
	for rows.Next() {
		var st Stream
		if err := rows.Scan(&st.ID, &st.Creator, &st.Title, &st.Status, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan stream: %w", err)
		}
		streams = append(streams, st)
	}
	return streams, rows.Err()
}
