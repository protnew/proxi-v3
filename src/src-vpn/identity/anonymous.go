package identity

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

// DisposableIdentity is a one-time-use identity without recovery mnemonic.
type DisposableIdentity struct {
	PrivKey *btcec.PrivateKey
	PubKey  *btcec.PublicKey
	Npub    string
	Nsec    string
	Created int64
	Expires int64
}

// GenerateDisposableIdentity creates a one-time identity pair.
func GenerateDisposableIdentity() (*DisposableIdentity, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate privkey: %w", err)
	}
	pubKey := privKey.PubKey()

	return &DisposableIdentity{
		PrivKey: privKey,
		PubKey:  pubKey,
		Npub:    PubKeyToNpub(pubKey),
		Nsec:    PrivKeyToNsec(privKey),
		Created: time.Now().Unix(),
		Expires: 0,
	}, nil
}

// IsDisposable returns true — these identities are always disposable.
func (d *DisposableIdentity) IsDisposable() bool {
	return true
}

// ExpiresIn sets the identity to expire after the given duration.
func (d *DisposableIdentity) ExpiresIn(dur time.Duration) {
	d.Expires = time.Now().Add(dur).Unix()
}

// IsExpired checks if the disposable identity has expired.
func (d *DisposableIdentity) IsExpired() bool {
	if d.Expires == 0 {
		return false
	}
	return time.Now().Unix() > d.Expires
}

// RandomBytes generates cryptographically secure random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
