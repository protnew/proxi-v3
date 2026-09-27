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
type issuedRec struct {
	Exp  int64  `json:"exp"`
	Npub string `json:"npub"`
}

type ExitAuth struct {
	mu     sync.Mutex
	path   string
	Allow  map[string]int64     `json:"allow"`
	Used   map[string]int64     `json:"used"`
	Issued map[string]issuedRec `json:"issued"`
	Max    int
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
	if a.Issued == nil {
		a.Issued = map[string]issuedRec{}
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

func (a *ExitAuth) Issue(token string, exp int64, bindNpub string) error {
	if token == "" || bindNpub == "" {
		return fmt.Errorf("token and npub required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Issued == nil {
		a.Issued = map[string]issuedRec{}
	}
	a.Issued[token] = issuedRec{Exp: exp, Npub: bindNpub}
	return a.save()
}

// Admit checks Schnorr, then the donor record. Wire exp is ignored.
func (a *ExitAuth) Admit(npub, token string, exp int64, sigHex string, now int64) error {
	_ = exp
	if targetDenied(npub) {
		return fmt.Errorf("npub is not a target")
	}
	if err := verifyTokenSig(npub, token, sigHex); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prune(now)
	if a.Issued == nil {
		return fmt.Errorf("unknown token")
	}
	rec, ok := a.Issued[token]
	if !ok {
		return fmt.Errorf("unknown token")
	}
	if rec.Npub != npub {
		return fmt.Errorf("token bound to another npub")
	}
	if now > rec.Exp {
		return fmt.Errorf("token expired")
	}
	if _, used := a.Used[token]; used {
		return fmt.Errorf("token spent")
	}
	a.Used[token] = rec.Exp
	if a.Allow == nil {
		a.Allow = map[string]int64{}
	}
	a.Allow[npub] = now
	return a.save()
}

func (a *ExitAuth) prune(now int64) {
	if a.Used == nil {
		a.Used = map[string]int64{}
	}
	for tok, exp := range a.Used {
		if exp > 0 && now > exp {
			delete(a.Used, tok)
		}
	}
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
