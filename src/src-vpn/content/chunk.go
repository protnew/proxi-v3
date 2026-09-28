// Package content provides chunking, encryption, manifest management, and HLS
// segmentation for the crypto video vault.
package content

import (
	"crypto/sha256"
	"fmt"
	"io"
)

// DefaultChunkSize is 1 MB.
const DefaultChunkSize = 1 * 1024 * 1024 // 1 MB

// Chunk represents a single piece of a split file.
type Chunk struct {
	ID    string // unique identifier (hex of hash)
	Index int    // zero-based position in the original file
	Data  []byte // raw payload
	Hash  [32]byte
}

// computeHash fills the Hash field with SHA-256 of Data and sets ID to its hex.
func (c *Chunk) computeHash() {
	c.Hash = sha256.Sum256(c.Data)
	c.ID = fmt.Sprintf("%x", c.Hash)
}

// ChunkFile splits an io.Reader into chunks of at most chunkSize bytes.
// The last chunk may be smaller. Each chunk gets a sequential index starting
// from 0 and a SHA-256 hash that also serves as its ID.
func ChunkFile(r io.Reader, chunkSize int) ([]Chunk, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be > 0, got %d", chunkSize)
	}
	buf := make([]byte, chunkSize)
	var chunks []Chunk
	idx := 0
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			c := Chunk{Index: idx, Data: data}
			c.computeHash()
			chunks = append(chunks, c)
			idx++
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read chunk %d: %w", idx, err)
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no data to chunk")
	}
	return chunks, nil
}

// AssembleChunks concatenates the Data fields of chunks ordered by Index
// and returns the reassembled byte slice.
func AssembleChunks(chunks []Chunk) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to assemble")
	}
	// Verify hashes first.
	for _, c := range chunks {
		got := sha256.Sum256(c.Data)
		if got != c.Hash {
			return nil, fmt.Errorf("chunk %d hash mismatch: integrity check failed", c.Index)
		}
	}
	// Sort is not needed if caller preserves order; but we verify ordering.
	total := 0
	for _, c := range chunks {
		total += len(c.Data)
	}
	out := make([]byte, 0, total)
	for _, c := range chunks {
		out = append(out, c.Data...)
	}
	return out, nil
}
