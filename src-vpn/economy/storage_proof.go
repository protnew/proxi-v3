package economy

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Challenge represents a proof-of-storage challenge sent to a node.
type Challenge struct {
	ChunkHash string    // hex-encoded SHA-256 of the original chunk
	Nonce     string    // random nonce for this challenge
	Timestamp time.Time // when the challenge was generated
}

// GenerateChallenge creates a new storage challenge for the given chunk hash.
func GenerateChallenge(chunkHash string) Challenge {
	nonceBytes := make([]byte, 32)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	return Challenge{
		ChunkHash: chunkHash,
		Nonce:     nonce,
		Timestamp: time.Now(),
	}
}

// GenerateProofData creates a proof response for a given challenge and original data.
// The proof is: SHA256(originalData || nonce || chunkHash).
// The node must possess originalData to compute this.
func GenerateProofData(challenge Challenge, originalData []byte) []byte {
	h := sha256.New()
	h.Write(originalData)
	h.Write([]byte(challenge.Nonce))
	h.Write([]byte(challenge.ChunkHash))
	return h.Sum(nil)
}

// VerifyProof checks that a node's proof is valid for the given challenge and original data.
// It re-computes the expected hash and compares against proofData.
func VerifyProof(challenge Challenge, proofData []byte, originalData []byte) bool {
	expected := GenerateProofData(challenge, originalData)
	if len(proofData) != len(expected) {
		return false
	}
	for i := range expected {
		if proofData[i] != expected[i] {
			return false
		}
	}
	return true
}

// ChunkHash computes the hex-encoded SHA-256 of data, used as a chunk identifier.
func ChunkHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
