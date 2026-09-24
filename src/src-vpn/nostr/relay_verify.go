package nostr

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func verifyEventSig(e *Event) error {
	if e.Sig == "" {
		return errors.New("missing sig")
	}
	sigBytes, err := hex.DecodeString(e.Sig)
	if err != nil {
		return errors.New("bad sig hex")
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return errors.New("bad schnorr sig")
	}
	pkBytes, err := hex.DecodeString(e.PubKey)
	if err != nil || len(pkBytes) != 32 {
		return errors.New("bad pubkey hex")
	}
	pk, err := schnorr.ParsePubKey(pkBytes)
	if err != nil {
		return errors.New("bad x-only pubkey")
	}
	idBytes, err := hex.DecodeString(e.ID)
	if err != nil {
		return errors.New("bad id hex")
	}
	if !sig.Verify(idBytes, pk) {
		return errors.New("signature verification failed")
	}
	return nil
}

// containsSensitiveKind reports whether the filter asks for private kinds.
func containsSensitiveKind(kinds []int) bool {
	for _, k := range kinds {
		if k == 4 || k == 30090 {
			return true
		}
	}
	return false
}

// deliveryAllowed: private-kind events reach only the author and p-tag peers.
func deliveryAllowed(c *Client, e *Event) bool {
	if e.Kind != 4 && e.Kind != 30090 {
		return true
	}
	if c.AuthPubkey == "" {
		return false
	}
	if strings.EqualFold(e.PubKey, c.AuthPubkey) {
		return true
	}
	for _, t := range e.Tags {
		if len(t) >= 2 && t[0] == "p" && strings.EqualFold(t[1], c.AuthPubkey) {
			return true
		}
	}
	return false
}
