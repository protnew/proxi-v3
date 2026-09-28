package vpn

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// FieldSealer is the sealField contract (identity.Store.sealField).
type FieldSealer interface {
	Seal(plain string) (string, error)
	Open(sealed string) (string, error)
}

// CertRoll persists WT cert+key until the next roll and publishes current+next hashes.
type CertRoll struct {
	CurrentHash string
	NextHash    string
	SealedKey   string
	SealedCert  string
}

func (c *CertRoll) Store(sealer FieldSealer, certDER, keyPEM []byte) error {
	if sealer == nil {
		return fmt.Errorf("sealer required")
	}
	sum := sha256.Sum256(certDER)
	c.NextHash = c.CurrentHash
	c.CurrentHash = hex.EncodeToString(sum[:])
	var err error
	c.SealedKey, err = sealer.Seal(string(keyPEM))
	if err != nil {
		return err
	}
	c.SealedCert, err = sealer.Seal(string(certDER))
	return err
}

func (c *CertRoll) PublishedHashes() (current, next string) {
	return c.CurrentHash, c.NextHash
}
