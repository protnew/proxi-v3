package vpn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func TestD1_ChainFallsThrough(t *testing.T) {
	var hit string
	ch := DesktopChain(map[string]func(context.Context, string) (net.Conn, error){
		"wt": func(ctx context.Context, target string) (net.Conn, error) {
			hit = "wt"
			a, b := net.Pipe()
			b.Close()
			return a, nil
		},
	})
	_, name, err := ch.Dial(context.Background(), "1.1.1.1:443")
	if err != nil || name != "wt" || hit != "wt" {
		t.Fatalf("name=%s hit=%s err=%v", name, hit, err)
	}
	if !LoopbackEgressIsNotProof("127.0.0.1") {
		t.Fatal("loopback must not count as egress proof")
	}
}

func TestD3_DropAndFakeDNS(t *testing.T) {
	fwd, mode := DropUDP([]byte{1, 2, 3})
	if fwd != nil || mode != "dropped" {
		t.Fatalf("fwd=%v mode=%s", fwd, mode)
	}
	q := make([]byte, 12)
	q[0], q[1] = 0x12, 0x34
	ans := FakeDNS(q)
	if ans[2]&0x80 == 0 {
		t.Fatal("QR bit not set")
	}
	tel := Phase0Telemetry("dc")
	if tel.UDPMode != "dropped" {
		t.Fatal(tel)
	}
}

type memSeal struct{ m map[string]string }

func (s *memSeal) Seal(plain string) (string, error) {
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m["k"] = "sealed:" + plain
	return s.m["k"], nil
}
func (s *memSeal) Open(sealed string) (string, error) { return sealed, nil }

func TestD7_CertHashesOverlap(t *testing.T) {
	var c CertRoll
	s := &memSeal{}
	if err := c.Store(s, []byte("cert-a"), []byte("key-a")); err != nil {
		t.Fatal(err)
	}
	cur, next := c.PublishedHashes()
	if cur == "" || next != "" {
		t.Fatalf("first roll cur=%s next=%s", cur, next)
	}
	if err := c.Store(s, []byte("cert-b"), []byte("key-b")); err != nil {
		t.Fatal(err)
	}
	cur2, next2 := c.PublishedHashes()
	if next2 != cur || cur2 == cur {
		t.Fatalf("overlap lost cur2=%s next2=%s prev=%s", cur2, next2, cur)
	}
	if len(c.SealedKey) < 8 || c.SealedKey[:7] != "sealed:" {
		t.Fatal("key not sealed")
	}
}

func TestDAUTH_TokenAndAllowlist(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	npub := hexXOnly(priv)
	token := "once"
	sum := sha256Token(token, npub)
	sig, err := schnorr.Sign(priv, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "allow.json")
	a := NewExitAuth(path)
	now := time.Now().Unix()
	if err := a.Admit(npub, token, now+60, hexSig(sig), now); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(npub, token, now+60, hexSig(sig), now); err == nil {
		t.Fatal("spent token must fail")
	}
	if err := a.AllowNpub(npub); err != nil {
		t.Fatal(err)
	}
	sum2 := sha256Token("other", npub)
	sig2, err := schnorr.Sign(priv, sum2[:])
	if err != nil {
		t.Fatal(err)
	}
	b := NewExitAuth(path)
	if err := b.Admit(npub, "other", now-10, hexSig(sig2), now); err != nil {
		t.Fatal("allowlist must skip expiry", err)
	}
}

func hexXOnly(priv *btcec.PrivateKey) string {
	return hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
}
func hexSig(sig *schnorr.Signature) string { return hex.EncodeToString(sig.Serialize()) }
func sha256Token(token, npub string) [32]byte {
	return sha256.Sum256([]byte(token + "|" + npub))
}
