package chat

import "testing"

func TestLooksLikeClientCiphertext(t *testing.T) {
	if LooksLikeClientCiphertext("hello") {
		t.Fatal("plaintext must not look like ciphertext")
	}
	if !LooksLikeClientCiphertext("nip44:abc") {
		t.Fatal("nip44 prefix must be detected")
	}
	if !LooksLikeClientCiphertext("v1.a.b") {
		t.Fatal("legacy v1 prefix must be detected")
	}
}

func TestNormalizeE2EFlags_refusePlaintextClaim(t *testing.T) {
	m := &Message{Type: TypeChat, Text: "secret", Encrypted: true, IsE2E: true}
	m.NormalizeE2EFlags()
	if m.Encrypted || m.IsE2E {
		t.Fatal("plaintext must clear encrypted flags")
	}
}

func TestNormalizeE2EFlags_ciphertextSetsFlags(t *testing.T) {
	m := &Message{Type: TypeChat, Text: "nip44:deadbeef", Encrypted: false}
	m.NormalizeE2EFlags()
	if !m.Encrypted || !m.IsE2E {
		t.Fatal("ciphertext must set encrypted flags")
	}
}

func TestClaimedEncrypted(t *testing.T) {
	m := &Message{Encrypted: true}
	if !m.ClaimedEncrypted() {
		t.Fatal("expected claimed")
	}
	m2 := &Message{IsE2E: true}
	if !m2.ClaimedEncrypted() {
		t.Fatal("expected claimed via is_e2e")
	}
}
