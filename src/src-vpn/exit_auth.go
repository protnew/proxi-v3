package vpn

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// ExitAuth is D-AUTH-EXIT: one-time token bound to npub, plus a persistent allowlist.
type ExitAuth struct {
	mu    sync.Mutex
	path  string
	Allow map[string]int64 `json:"allow"`
	Used  map[string]int64 `json:"used"`
	Max   int
}

func NewExitAuth(path string) *ExitAuth {
	a := &ExitAuth{path: path, Allow: map[string]int64{}, Used: map[string]int64{}, Max: 32}
	if path == "" {
		return a
	}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, a)
	}
	if a.Allow == nil {
		a.Allow = map[string]int64{}
	}
	if a.Used == nil {
		a.Used = map[string]int64{}
	}
	if a.Max == 0 {
		a.Max = 32
	}
	return a
}

func (a *ExitAuth) save() error {
	if a.path == "" {
		return nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return os.WriteFile(a.path, b, 0o600)
}

func (a *ExitAuth) AllowNpub(npub string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Allow[npub] = time.Now().Unix()
	return a.save()
}

// Admit checks Schnorr(sig over token||npub) and either the allowlist or a fresh token.
func (a *ExitAuth) Admit(npub, token string, exp int64, sigHex string, now int64) error {
	if targetDenied(npub) {
		return fmt.Errorf("npub is not a target")
	}
	if err := verifyTokenSig(npub, token, sigHex); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.Allow)+len(a.Used) > a.Max*4 {
		return fmt.Errorf("session limit")
	}
	if _, ok := a.Allow[npub]; ok {
		return a.save()
	}
	if now > exp {
		return fmt.Errorf("token expired")
	}
	if _, used := a.Used[token]; used {
		return fmt.Errorf("token spent")
	}
	a.Used[token] = now
	return a.save()
}

func verifyTokenSig(npub, token, sigHex string) error {
	pkb, err := hex.DecodeString(npub)
	if err != nil || len(pkb) != 32 {
		return fmt.Errorf("bad npub")
	}
	pk, err := schnorr.ParsePubKey(pkb)
	if err != nil {
		return err
	}
	sigb, err := hex.DecodeString(sigHex)
	if err != nil {
		return err
	}
	sig, err := schnorr.ParseSignature(sigb)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(token + "|" + npub))
	if !sig.Verify(sum[:], pk) {
		return fmt.Errorf("schnorr verify failed")
	}
	return nil
}
