package crypto

import "testing"

func TestCRYP012_Ed25519SignVerify(t *testing.T) {
	pub, priv, err := GenerateEd25519()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignMessageEd25519(priv, "alice", "bob", "hello", 123)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyMessageEd25519(pub, "alice", "bob", "hello", 123, sig) {
		t.Fatal("valid sig rejected")
	}
	if VerifyMessageEd25519(pub, "alice", "bob", "HELLO", 123, sig) {
		t.Fatal("tampered text accepted")
	}
	if VerifyMessageEd25519(pub, "eve", "bob", "hello", 123, sig) {
		t.Fatal("tampered from accepted")
	}
}
