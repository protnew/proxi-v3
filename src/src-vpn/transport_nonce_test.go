package vpn

import (
	"bytes"
	"testing"
)

func TestF12_NonceIsCounterNotRandom(t *testing.T) {
	n1 := counterNonce(12, 1)
	n2 := counterNonce(12, 2)
	n1b := counterNonce(12, 1)
	if bytes.Equal(n1, n2) {
		t.Fatal("seq 1 and 2 produced the same nonce")
	}
	if !bytes.Equal(n1, n1b) {
		t.Fatal("same seq must be deterministic, not random")
	}
	if n2[11] <= n1[11] && bytes.Compare(n2, n1) <= 0 {
		t.Fatal("nonce must increase with seq")
	}
	key := bytes.Repeat([]byte{7}, 32)
	p1, err := encryptPacket(key, []byte("a"), 1)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := encryptPacket(key, []byte("a"), 2)
	if err != nil {
		t.Fatal(err)
	}
	nonce1 := p1[8:20]
	nonce2 := p2[8:20]
	if !bytes.Equal(nonce1, n1) || !bytes.Equal(nonce2, n2) {
		t.Fatalf("packet nonce not counter: %x %x", nonce1, nonce2)
	}
}
