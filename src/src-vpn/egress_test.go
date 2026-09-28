package vpn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"crypto/sha256"
	"encoding/hex"
)

// signAuthFrame builds a valid {npub,token,sig,exp} frame for tests.
func signAuthFrame(t *testing.T, priv *btcec.PrivateKey, token string, exp int64) string {
	t.Helper()
	npub := hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
	sum := sha256.Sum256([]byte(token + "|" + npub))
	sig, err := schnorr.Sign(priv, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	f, _ := json.Marshal(authFrame{Npub: npub, Token: token, Sig: hex.EncodeToString(sig.Serialize()), Exp: exp})
	return string(f)
}

// startTestEgress starts an egress listener on the given manager and returns
// the relay address plus cleanup.
func startTestEgress(t *testing.T, m *Manager) (addr string, cleanup func()) {
	t.Helper()
	if _, err := m.StartEgressListener(""); err != nil {
		t.Fatal(err)
	}
	// WT leg is skipped (no public bind); use the raw TCP listener address.
	m.mu.Lock()
	ln := m.egress.ln
	m.mu.Unlock()
	addr = ln.Addr().String()
	return addr, m.StopEgress
}

func TestEgress_AuthAdmitDeny(t *testing.T) {
	m := testManager(t)
	addr, stop := startTestEgress(t, m)
	defer stop()

	priv, _ := btcec.NewPrivateKey()
	npub := hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
	if err := m.exitAuth().Issue("tok1", time.Now().Unix()+60, npub); err != nil {
		t.Fatal(err)
	}

	// Deny: garbage first frame.
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write([]byte("not-json\n"))
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if n, _ := c.Read(buf); n > 0 && strings.HasPrefix(string(buf[:n]), "ok") {
		t.Fatal("garbage frame admitted")
	}
	c.Close()

	// Admit: valid signed frame -> "ok\n".
	c, err = net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write([]byte(signAuthFrame(t, priv, "tok1", time.Now().Unix()+60) + "\n"))
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil || line != "ok\n" {
		t.Fatalf("expected ok, got %q err=%v", line, err)
	}
	c.Close()
}

func TestEgress_ACLDeniesPrivate(t *testing.T) {
	m := testManager(t)
	addr, stop := startTestEgress(t, m)
	defer stop()

	priv, _ := btcec.NewPrivateKey()
	npub := hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
	if err := m.exitAuth().Issue("tok2", time.Now().Unix()+60, npub); err != nil {
		t.Fatal(err)
	}

	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _ = c.Write([]byte(signAuthFrame(t, priv, "tok2", time.Now().Unix()+60) + "\n"))
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(c)
	if line, _ := r.ReadString('\n'); line != "ok\n" {
		t.Fatalf("auth not admitted: %q", line)
	}
	_, _ = fmt.Fprintf(c, "CONNECT 169.254.169.254:80\n")
	line, err := r.ReadString('\n')
	if err != nil || line != "denied\n" {
		t.Fatalf("metadata target not denied: %q err=%v", line, err)
	}
}

// Unit-level ACL matrix — no sockets needed.
func TestACL_DeniesPrivateAndMetadata(t *testing.T) {
	for _, target := range []string{
		"127.0.0.1:80", "10.0.0.1:443", "192.168.1.1:22", "169.254.169.254:80",
		"localhost:22", "0.0.0.0:1", "[::1]:443",
	} {
		if _, denied := aclResolveTarget(target); !denied {
			t.Fatalf("target %s not denied", target)
		}
	}
	if _, denied := aclResolveTarget("8.8.8.8:53"); denied {
		t.Fatal("public DNS denied")
	}
}

func TestEgress_ReplayTokenSpent(t *testing.T) {
	m := testManager(t)
	addr, stop := startTestEgress(t, m)
	defer stop()

	priv, _ := btcec.NewPrivateKey()
	npub := hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
	if err := m.exitAuth().Issue("tok3", time.Now().Unix()+60, npub); err != nil {
		t.Fatal(err)
	}
	frame := signAuthFrame(t, priv, "tok3", time.Now().Unix()+60)

	for i, want := range []string{"ok\n", ""} {
		c, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = c.Write([]byte(frame + "\n"))
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		line, _ := bufio.NewReader(c).ReadString('\n')
		if i == 0 && line != want {
			t.Fatalf("first admit: %q", line)
		}
		if i == 1 && line == "ok\n" {
			t.Fatal("spent token admitted again (replay)")
		}
		c.Close()
	}
}
