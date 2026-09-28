package content

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// ChunkRef is a reference to a single chunk stored in the vault.
type ChunkRef struct {
	Hash          string `json:"hash"`           // SHA-256 hex of the original chunk data
	EncryptedHash string `json:"encrypted_hash"` // SHA-256 hex of the encrypted chunk data
	Size          int    `json:"size"`           // size of encrypted chunk in bytes
}

// ContentManifest describes a piece of content stored in the vault.
type ContentManifest struct {
	ID                 string     `json:"id"`                  // unique manifest ID (UUID-like)
	FileName           string     `json:"file_name"`           // original file name
	FileSize           int64      `json:"file_size"`           // original file size in bytes
	ChunkSize          int        `json:"chunk_size"`          // chunk size used for splitting
	TotalChunks        int        `json:"total_chunks"`        // number of chunks
	ErasureK           int        `json:"erasure_k,omitempty"` // erasure coding data shards
	ErasureN           int        `json:"erasure_n,omitempty"` // erasure coding total shards
	EncryptedMasterKey []byte     `json:"encrypted_master_key,omitempty"`
	Chunks             []ChunkRef `json:"chunks"`
	CreatedAt          time.Time  `json:"created_at"`
}

// CreateManifest builds a ContentManifest from the given parameters.
// chunks should be the original (plaintext) chunks; encryptedChunks should
// be the corresponding encrypted chunks (same order, same length).
func CreateManifest(id, fileName string, fileSize int64, chunkSize int,
	chunks []Chunk, encryptedChunks []EncryptedChunk, erasureK, erasureN int,
	encryptedMasterKey []byte) (*ContentManifest, error) {

	if len(chunks) != len(encryptedChunks) {
		return nil, fmt.Errorf("chunk count mismatch: %d original vs %d encrypted",
			len(chunks), len(encryptedChunks))
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided")
	}

	refs := make([]ChunkRef, len(chunks))
	for i := range chunks {
		encHash := sha256.Sum256(encryptedChunks[i].Data)
		refs[i] = ChunkRef{
			Hash:          chunks[i].ID,
			EncryptedHash: fmt.Sprintf("%x", encHash),
			Size:          len(encryptedChunks[i].Data),
		}
	}

	return &ContentManifest{
		ID:                 id,
		FileName:           fileName,
		FileSize:           fileSize,
		ChunkSize:          chunkSize,
		TotalChunks:        len(chunks),
		ErasureK:           erasureK,
		ErasureN:           erasureN,
		EncryptedMasterKey: encryptedMasterKey,
		Chunks:             refs,
		CreatedAt:          time.Now().UTC(),
	}, nil
}

// VerifyManifest checks integrity of the manifest structure:
//   - chunk count matches TotalChunks
//   - all hashes are non-empty
//   - no duplicate chunk hashes
func (m *ContentManifest) VerifyManifest() error {
	if m.ID == "" {
		return fmt.Errorf("manifest ID is empty")
	}
	if m.FileName == "" {
		return fmt.Errorf("file name is empty")
	}
	if m.TotalChunks <= 0 {
		return fmt.Errorf("total chunks must be > 0, got %d", m.TotalChunks)
	}
	if len(m.Chunks) != m.TotalChunks {
		return fmt.Errorf("chunk count mismatch: TotalChunks=%d, len(Chunks)=%d",
			m.TotalChunks, len(m.Chunks))
	}
	seen := make(map[string]struct{}, len(m.Chunks))
	for i, ref := range m.Chunks {
		if ref.Hash == "" {
			return fmt.Errorf("chunk %d has empty hash", i)
		}
		if ref.EncryptedHash == "" {
			return fmt.Errorf("chunk %d has empty encrypted hash", i)
		}
		if ref.Size <= 0 {
			return fmt.Errorf("chunk %d has invalid size %d", i, ref.Size)
		}
		if _, dup := seen[ref.Hash]; dup {
			return fmt.Errorf("duplicate chunk hash at index %d: %s", i, ref.Hash)
		}
		seen[ref.Hash] = struct{}{}
	}
	return nil
}

// Serialize converts the manifest to JSON bytes.
func (m *ContentManifest) Serialize() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize manifest: %w", err)
	}
	return data, nil
}

// DeserializeManifest parses JSON bytes into a ContentManifest.
func DeserializeManifest(data []byte) (*ContentManifest, error) {
	var m ContentManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("deserialize manifest: %w", err)
	}
	return &m, nil
}
