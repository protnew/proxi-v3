package vpn

import (
	"fmt"
	"testing"
	"time"
)

func TestSendToPeer_LoopbackRoundTrip(t *testing.T) {
	got := make(chan []byte, 1)
	b, err := NewUserspaceVPN(UserspaceConfig{
		ListenAddr: "127.0.0.1:0",
		OnReceive: func(_ string, plain []byte) {
			got <- append([]byte(nil), plain...)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewUserspaceVPN(UserspaceConfig{ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Stop(); _ = b.Stop() })

	bID := b.GetPublicKey()[:16]
	if err := a.AddPeer(PeerInfo{ID: bID, PublicKey: b.GetPublicKey(), Endpoint: fmt.Sprintf("127.0.0.1:%d", b.GetLocalPort())}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddPeer(PeerInfo{ID: a.GetPublicKey()[:16], PublicKey: a.GetPublicKey(), Endpoint: fmt.Sprintf("127.0.0.1:%d", a.GetLocalPort())}); err != nil {
		t.Fatal(err)
	}
	if err := a.SendToPeer(bID, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-got:
		if string(msg) != "ping" {
			t.Fatalf("plaintext %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no plaintext")
	}
}

func TestHandshake_CompletesBetweenPeers(t *testing.T) {
	a, err := NewUserspaceVPN(UserspaceConfig{ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewUserspaceVPN(UserspaceConfig{ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Stop(); _ = b.Stop() })
	if err := a.AddPeer(PeerInfo{ID: b.GetPublicKey()[:16], PublicKey: b.GetPublicKey(), Endpoint: fmt.Sprintf("127.0.0.1:%d", b.GetLocalPort())}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddPeer(PeerInfo{ID: a.GetPublicKey()[:16], PublicKey: a.GetPublicKey(), Endpoint: fmt.Sprintf("127.0.0.1:%d", a.GetLocalPort())}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if handshakeDone(a, b.GetPublicKey()[:16]) && handshakeDone(b, a.GetPublicKey()[:16]) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("handshake did not complete")
}

func handshakeDone(u *UserspaceVPN, id string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	s := u.peers[id]
	return s != nil && s.handshakeDone.Load()
}
