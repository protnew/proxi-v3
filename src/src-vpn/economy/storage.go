package economy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ProofOfStorage verifies that a peer is storing a chunk.
// Challenge: hash(chunkData) must match expected hash.
type ProofOfStorage struct{}

func (p *ProofOfStorage) Verify(chunkData []byte, expectedHash string) bool {
	h := sha256.Sum256(chunkData)
	return hex.EncodeToString(h[:]) == expectedHash
}

// Challenge generates a random challenge for a storage provider.
func (p *ProofOfStorage) Challenge(chunkHash string) string {
	return fmt.Sprintf("prove-storage:%s", chunkHash)
}
