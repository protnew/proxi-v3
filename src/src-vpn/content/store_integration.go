package content

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// EnsureContentTables creates the content_manifests and content_catalog tables
// in the given SQLite database if they do not already exist. It is idempotent
// and safe to call on every startup.
func EnsureContentTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS content_manifests (
			id               TEXT PRIMARY KEY,
			file_name        TEXT NOT NULL DEFAULT '',
			file_size        INTEGER NOT NULL DEFAULT 0,
			chunk_size       INTEGER NOT NULL DEFAULT 0,
			total_chunks     INTEGER NOT NULL DEFAULT 0,
			erasure_k        INTEGER NOT NULL DEFAULT 0,
			erasure_n        INTEGER NOT NULL DEFAULT 0,
			encrypted_master_key BLOB,
			chunks_json      TEXT NOT NULL DEFAULT '[]',
			created_at       INTEGER NOT NULL DEFAULT 0,
			payload          BLOB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_content_manifests_file_name ON content_manifests(file_name)`,

		`CREATE TABLE IF NOT EXISTS content_catalog (
			id           TEXT PRIMARY KEY,
			title        TEXT NOT NULL DEFAULT '',
			type         TEXT NOT NULL DEFAULT '',
			size         INTEGER NOT NULL DEFAULT 0,
			uploader     TEXT NOT NULL DEFAULT '',
			manifest_id  TEXT NOT NULL DEFAULT '',
			access       TEXT NOT NULL DEFAULT 'public',
			version      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL DEFAULT 0,
			updated_at   INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_content_catalog_uploader ON content_catalog(uploader)`,
		`CREATE INDEX IF NOT EXISTS idx_content_catalog_type     ON content_catalog(type)`,
		`CREATE INDEX IF NOT EXISTS idx_content_catalog_access   ON content_catalog(access)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("ensure content tables: exec %q: %w", q, err)
		}
	}
	return nil
}

// StoredManifest is the on-disk representation of a ContentManifest plus an
// optional encrypted payload blob. Chunks are serialised to JSON.
type StoredManifest struct {
	ID                 string
	FileName           string
	FileSize           int64
	ChunkSize          int
	TotalChunks        int
	ErasureK           int
	ErasureN           int
	EncryptedMasterKey []byte
	Chunks             []ChunkRef
	CreatedAt          int64
	Payload            []byte // encrypted chunk data map serialised, or raw blob
}

// SaveManifest inserts or replaces a StoredManifest row identified by its ID.
func SaveManifest(db *sql.DB, m *StoredManifest) error {
	chunksJSON, err := json.Marshal(m.Chunks)
	if err != nil {
		return fmt.Errorf("save manifest: marshal chunks: %w", err)
	}
	encKey := m.EncryptedMasterKey
	if encKey == nil {
		encKey = []byte{}
	}
	payload := m.Payload
	if payload == nil {
		payload = []byte{}
	}

	_, err = db.Exec(
		`INSERT INTO content_manifests
			(id, file_name, file_size, chunk_size, total_chunks,
			 erasure_k, erasure_n, encrypted_master_key, chunks_json,
			 created_at, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   file_name = excluded.file_name,
		   file_size = excluded.file_size,
		   chunk_size = excluded.chunk_size,
		   total_chunks = excluded.total_chunks,
		   erasure_k = excluded.erasure_k,
		   erasure_n = excluded.erasure_n,
		   encrypted_master_key = excluded.encrypted_master_key,
		   chunks_json = excluded.chunks_json,
		   created_at = excluded.created_at,
		   payload = excluded.payload`,
		m.ID, m.FileName, m.FileSize, m.ChunkSize, m.TotalChunks,
		m.ErasureK, m.ErasureN, encKey, string(chunksJSON),
		m.CreatedAt, payload,
	)
	if err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	return nil
}

// LoadManifest retrieves a StoredManifest by ID. Returns sql.ErrNoRows if absent.
func LoadManifest(db *sql.DB, id string) (*StoredManifest, error) {
	row := db.QueryRow(
		`SELECT id, file_name, file_size, chunk_size, total_chunks,
		        erasure_k, erasure_n, encrypted_master_key, chunks_json,
		        created_at, payload
		 FROM content_manifests WHERE id = ?`, id)

	m := &StoredManifest{}
	var chunksJSON string
	var encKey, payload []byte
	if err := row.Scan(
		&m.ID, &m.FileName, &m.FileSize, &m.ChunkSize, &m.TotalChunks,
		&m.ErasureK, &m.ErasureN, &encKey, &chunksJSON,
		&m.CreatedAt, &payload,
	); err != nil {
		return nil, fmt.Errorf("load manifest %q: %w", id, err)
	}

	if len(encKey) > 0 {
		m.EncryptedMasterKey = encKey
	}
	if len(payload) > 0 {
		m.Payload = payload
	}
	if chunksJSON != "" {
		if err := json.Unmarshal([]byte(chunksJSON), &m.Chunks); err != nil {
			return nil, fmt.Errorf("load manifest %q: unmarshal chunks: %w", id, err)
		}
	}
	if m.Chunks == nil {
		m.Chunks = []ChunkRef{}
	}
	return m, nil
}

// DeleteManifest removes a manifest row by ID.
func DeleteManifest(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM content_manifests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete manifest %q: %w", id, err)
	}
	return nil
}

// SaveCatalogEntry inserts or replaces a ContentEntry row. It mirrors the
// Catalog.Add/Update semantics at the persistence layer.
func SaveCatalogEntry(db *sql.DB, e *ContentEntry) error {
	_, err := db.Exec(
		`INSERT INTO content_catalog
			(id, title, type, size, uploader, manifest_id,
			 access, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   title = excluded.title,
		   type = excluded.type,
		   size = excluded.size,
		   uploader = excluded.uploader,
		   manifest_id = excluded.manifest_id,
		   access = excluded.access,
		   version = excluded.version,
		   updated_at = excluded.updated_at`,
		e.ID, e.Title, string(e.Type), e.Size, e.Uploader, e.ManifestID,
		e.Access, e.Version, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save catalog entry %q: %w", e.ID, err)
	}
	return nil
}

// LoadCatalogEntry retrieves a single ContentEntry by ID. Returns sql.ErrNoRows
// if absent.
func LoadCatalogEntry(db *sql.DB, id string) (*ContentEntry, error) {
	row := db.QueryRow(
		`SELECT id, title, type, size, uploader, manifest_id,
		        access, version, created_at, updated_at
		 FROM content_catalog WHERE id = ?`, id)

	var e ContentEntry
	var ctype string
	if err := row.Scan(
		&e.ID, &e.Title, &ctype, &e.Size, &e.Uploader, &e.ManifestID,
		&e.Access, &e.Version, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("load catalog entry %q: %w", id, err)
	}
	e.Type = ContentType(ctype)
	return &e, nil
}

// DeleteCatalogEntry removes a catalog entry by ID.
func DeleteCatalogEntry(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM content_catalog WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete catalog entry %q: %w", id, err)
	}
	return nil
}

// ListCatalogEntries returns catalog entries matching the filter with
// pagination, ordered by ID (matching Catalog.List semantics).
func ListCatalogEntries(db *sql.DB, filter CatalogFilter) ([]ContentEntry, error) {
	q := `SELECT id, title, type, size, uploader, manifest_id,
	             access, version, created_at, updated_at
	      FROM content_catalog WHERE 1=1`
	args := []interface{}{}
	if filter.Type != "" {
		q += " AND type = ?"
		args = append(args, string(filter.Type))
	}
	if filter.Uploader != "" {
		q += " AND uploader = ?"
		args = append(args, filter.Uploader)
	}
	if filter.Access != "" {
		q += " AND access = ?"
		args = append(args, filter.Access)
	}
	q += " ORDER BY id ASC"
	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			q += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	} else if filter.Offset > 0 {
		// SQLite requires a LIMIT when OFFSET is used.
		q += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list catalog entries: %w", err)
	}
	defer rows.Close()

	var result []ContentEntry
	for rows.Next() {
		var e ContentEntry
		var ctype string
		if err := rows.Scan(
			&e.ID, &e.Title, &ctype, &e.Size, &e.Uploader, &e.ManifestID,
			&e.Access, &e.Version, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list catalog entries: scan: %w", err)
		}
		e.Type = ContentType(ctype)
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list catalog entries: rows: %w", err)
	}
	return result, nil
}
