package content

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/unkillable-messenger/vpn/storage"
)

// UploadPipeline orchestrates: file → chunks → encrypt → erasure → distribute.
type UploadPipeline struct {
	ChunkSize   int
	ErasureK    int // data shards
	ErasureN    int // total shards (data + parity)
	MasterKey   [32]byte
	encoder     *storage.ErasureEncoder
}

// NewUploadPipeline creates a pipeline with the given chunk size and erasure coding parameters.
// If k < n, erasure coding is enabled. If k == n or k <= 0, erasure coding is disabled.
func NewUploadPipeline(chunkSize, k, n int) *UploadPipeline {
	p := &UploadPipeline{
		ChunkSize: chunkSize,
		ErasureK:  k,
		ErasureN:  n,
	}
	// Initialize erasure encoder if parameters are valid and k < n
	if k > 0 && n > k {
		enc, err := storage.NewEncoder(k, n-k)
		if err == nil {
			p.encoder = enc
		}
	}
	return p
}

// ProcessUpload reads data from the reader, chunks it, encrypts each chunk,
// optionally erasure-codes, and returns a ContentManifest.
func (p *UploadPipeline) ProcessUpload(filename string, data io.Reader) (*ContentManifest, error) {
	// 1. Read all data
	allData, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	// 2. Handle empty file
	if len(allData) == 0 {
		return &ContentManifest{
			ID:        fmt.Sprintf("%x", sha256.Sum256(nil)),
			FileName:  filename,
			FileSize:  0,
			ChunkSize: p.ChunkSize,
			TotalChunks: 0,
			ErasureK:  p.ErasureK,
			ErasureN:  p.ErasureN,
			Chunks:    []ChunkRef{},
		}, nil
	}

	// 3. Generate master key if not already set
	if p.MasterKey == [32]byte{} {
		key, err := GenerateMasterKey()
		if err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		p.MasterKey = key
	}

	// 4. Chunk the data
	chunks, err := ChunkFile(bytes.NewReader(allData), p.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("chunk file: %w", err)
	}

	// 5. Encrypt each chunk
	encryptedChunks := make([]EncryptedChunk, len(chunks))
	// Store erasure-decoded (but still encrypted) data for manifest
	encryptedDataMap := make(map[int][]byte) // index → encrypted chunk data
	for i, c := range chunks {
		ec, err := EncryptChunk(p.MasterKey, c)
		if err != nil {
			return nil, fmt.Errorf("encrypt chunk %d: %w", i, err)
		}
		encryptedChunks[i] = ec
		encryptedDataMap[i] = ec.Data
	}

	// 6. Erasure encode each encrypted chunk if encoder is available
	if p.encoder != nil {
		for i, ec := range encryptedChunks {
			shards, err := p.encoder.Encode(ec.Data)
			if err != nil {
				return nil, fmt.Errorf("erasure encode chunk %d: %w", i, err)
			}
			_ = shards // In a real system, shards would be distributed to storage nodes
		}
	}

	// 7. Create manifest
	manifestID := fmt.Sprintf("%x", sha256.Sum256(allData))
	manifest, err := CreateManifest(
		manifestID,
		filename,
		int64(len(allData)),
		p.ChunkSize,
		chunks,
		encryptedChunks,
		p.ErasureK,
		p.ErasureN,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create manifest: %w", err)
	}

	return manifest, nil
}

// ProcessDownload takes a manifest and a map of chunk index → encrypted data,
// decrypts each chunk, verifies hashes, and reassembles the original data.
func (p *UploadPipeline) ProcessDownload(manifest *ContentManifest, encryptedChunks map[int][]byte) ([]byte, error) {
	// Handle empty file
	if manifest.TotalChunks == 0 {
		return []byte{}, nil
	}

	if len(encryptedChunks) != manifest.TotalChunks {
		return nil, fmt.Errorf("expected %d chunks, got %d", manifest.TotalChunks, len(encryptedChunks))
	}

	// 1. Decrypt each chunk
	decryptedChunks := make([]Chunk, manifest.TotalChunks)
	for i := 0; i < manifest.TotalChunks; i++ {
		encData, ok := encryptedChunks[i]
		if !ok {
			return nil, fmt.Errorf("missing encrypted chunk %d", i)
		}
		ec := EncryptedChunk{
			ID:    manifest.Chunks[i].Hash,
			Index: i,
			Data:  encData,
		}
		chunk, err := DecryptChunk(p.MasterKey, ec)
		if err != nil {
			return nil, fmt.Errorf("decrypt chunk %d: %w", i, err)
		}

		// 2. Verify hash matches manifest
		if chunk.ID != manifest.Chunks[i].Hash {
			return nil, fmt.Errorf("chunk %d hash mismatch: got %s, want %s", i, chunk.ID, manifest.Chunks[i].Hash)
		}

		decryptedChunks[i] = chunk
	}

	// 3. Assemble chunks
	result, err := AssembleChunks(decryptedChunks)
	if err != nil {
		return nil, fmt.Errorf("assemble chunks: %w", err)
	}

	return result, nil
}

// ProcessDownloadWithErasure attempts erasure recovery for missing chunks.
// missingIndices specifies which chunk indices are missing; the erasure decoder
// will attempt to recover them from the available shards.
func (p *UploadPipeline) ProcessDownloadWithErasure(manifest *ContentManifest, encryptedChunks map[int][]byte) ([]byte, error) {
	// Handle empty file
	if manifest.TotalChunks == 0 {
		return []byte{}, nil
	}

	// If all chunks present, use normal download
	if len(encryptedChunks) == manifest.TotalChunks {
		allPresent := true
		for i := 0; i < manifest.TotalChunks; i++ {
			if _, ok := encryptedChunks[i]; !ok {
				allPresent = false
				break
			}
		}
		if allPresent {
			return p.ProcessDownload(manifest, encryptedChunks)
		}
	}

	// If we have an erasure encoder, attempt recovery
	if p.encoder != nil {
		recovered := make(map[int][]byte)
		for k, v := range encryptedChunks {
			recovered[k] = v
		}

		// For each missing chunk, try to recover using erasure coding
		for i := 0; i < manifest.TotalChunks; i++ {
			if _, ok := recovered[i]; ok {
				continue
			}
			// We need the original encrypted data to recover.
			// In this simulation, we can't recover without stored shards,
			// so this remains a no-op placeholder for the full distributed system.
			return nil, fmt.Errorf("chunk %d missing and erasure recovery requires stored shards", i)
		}
		return p.ProcessDownload(manifest, recovered)
	}

	return nil, fmt.Errorf("missing chunks and no erasure encoder available")
}
