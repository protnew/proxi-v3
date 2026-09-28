package vpn

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func TestExitAuth_DeniesUnissuedExpiredAndForeign(t *testing.T) {
	owner, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	thief, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	ownerNpub := hexXOnly(owner)
	thiefNpub := hexXOnly(thief)
	now := time.Now().Unix()
	a := NewExitAuth("")
	if err := a.Admit(ownerNpub, "nope", now+60, signToken(t, owner, "nope", ownerNpub), now); err == nil {
		t.Fatal("unissued token admitted")
	}
	if err := a.Issue("gone", now-5, ownerNpub); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(ownerNpub, "gone", now+999, signToken(t, owner, "gone", ownerNpub), now); err == nil {
		t.Fatal("record expiry ignored")
	}
	if err := a.Issue("stolen", now+60, ownerNpub); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(thiefNpub, "stolen", now+60, signToken(t, thief, "stolen", thiefNpub), now); err == nil {
		t.Fatal("foreign key admitted stolen token")
	}
}

func TestExitAuth_AdmitAddsAllowlist(t *testing.T) {
	owner, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	npub := hexXOnly(owner)
	now := time.Now().Unix()
	a := NewExitAuth("")
	if err := a.Issue("once", now+60, npub); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(npub, "once", 0, signToken(t, owner, "once", npub), now); err != nil {
		t.Fatal(err)
	}
	if !a.IsAllowed(npub, now) {
		t.Fatalf("allowlist miss: %v", a.Allow)
	}
	if a.Allow[npub] <= now {
		t.Fatal("allow entry must be an expiry in the future")
	}
}

func signToken(t *testing.T, priv *btcec.PrivateKey, token, npub string) string {
	t.Helper()
	sum := sha256Token(token, npub)
	sig, err := schnorr.Sign(priv, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return hexSig(sig)
}
