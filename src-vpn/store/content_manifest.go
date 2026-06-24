package store

import (
	"log"
)

type ContentManifest struct {
	ID          string `json:"id"`
	OwnerNpub   string `json:"owner_npub"`
	ContentType string `json:"content_type"`
	Metadata    string `json:"metadata"`
	CreatedAt   int64  `json:"created_at"`
}

type ContentCatalog struct {
	ID           string `json:"id"`
	ManifestID   string `json:"manifest_id"`
	ChunkHash    string `json:"chunk_hash"`
	Size         int64  `json:"size"`
	Availability string `json:"availability"`
}

func (s *Store) SaveContentManifest(cm ContentManifest) error {
	_, err := s.db.Exec(`
		INSERT INTO content_manifests (id, owner_npub, content_type, metadata, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			metadata=excluded.metadata
	`, cm.ID, cm.OwnerNpub, cm.ContentType, cm.Metadata, cm.CreatedAt)
	if err != nil {
		log.Printf("SaveContentManifest error: %v", err)
	}
	return err
}

func (s *Store) GetContentManifest(id string) (*ContentManifest, error) {
	row := s.db.QueryRow("SELECT id, owner_npub, content_type, metadata, created_at FROM content_manifests WHERE id = ?", id)
	var cm ContentManifest
	if err := row.Scan(&cm.ID, &cm.OwnerNpub, &cm.ContentType, &cm.Metadata, &cm.CreatedAt); err != nil {
		return nil, err
	}
	return &cm, nil
}

func (s *Store) SaveContentCatalog(cc ContentCatalog) error {
	_, err := s.db.Exec(`
		INSERT INTO content_catalog (id, manifest_id, chunk_hash, size, availability)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			availability=excluded.availability
	`, cc.ID, cc.ManifestID, cc.ChunkHash, cc.Size, cc.Availability)
	if err != nil {
		log.Printf("SaveContentCatalog error: %v", err)
	}
	return err
}

func (s *Store) GetContentCatalogByManifest(manifestID string) ([]ContentCatalog, error) {
	rows, err := s.db.Query("SELECT id, manifest_id, chunk_hash, size, availability FROM content_catalog WHERE manifest_id = ?", manifestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ContentCatalog
	for rows.Next() {
		var cc ContentCatalog
		if err := rows.Scan(&cc.ID, &cc.ManifestID, &cc.ChunkHash, &cc.Size, &cc.Availability); err == nil {
			results = append(results, cc)
		}
	}
	return results, nil
}
