package crypto

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestDoubleRatchetConversation — Alice sends, Bob decrypts, Bob sends, Alice decrypts
// ---------------------------------------------------------------------------

func TestDoubleRatchetConversation(t *testing.T) {
	t.Parallel()

	// Shared secret agreed via X3DH (out of scope — we use a random 32 bytes here)
	var shared [32]byte
	copy(shared[:], []byte("test-shared-secret-32-bytes-pad!"))

	alice := NewRatchetState(shared[:], true)
	bob := NewRatchetState(shared[:], false)

	// Set each other's initial public key so the DH ratchet can work
	alice.RemotePub = bob.DHPub
	bob.RemotePub = alice.DHPub

	// --- Alice sends message 1 to Bob ---
	ct1, err := alice.RatchetEncrypt([]byte("Hello Bob!"))
	if err != nil {
		t.Fatalf("Alice encrypt 1: %v", err)
	}
	pt1, err := bob.RatchetDecrypt(ct1)
	if err != nil {
		t.Fatalf("Bob decrypt 1: %v", err)
	}
	if string(pt1) != "Hello Bob!" {
		t.Fatalf("msg1 mismatch: got %q", string(pt1))
	}

	// --- Alice sends message 2 to Bob ---
	ct2, err := alice.RatchetEncrypt([]byte("How are you?"))
	if err != nil {
		t.Fatalf("Alice encrypt 2: %v", err)
	}
	pt2, err := bob.RatchetDecrypt(ct2)
	if err != nil {
		t.Fatalf("Bob decrypt 2: %v", err)
	}
	if string(pt2) != "How are you?" {
		t.Fatalf("msg2 mismatch: got %q", string(pt2))
	}

	// --- Bob sends message to Alice ---
	ct3, err := bob.RatchetEncrypt([]byte("Hi Alice, I'm fine!"))
	if err != nil {
		t.Fatalf("Bob encrypt: %v", err)
	}
	pt3, err := alice.RatchetDecrypt(ct3)
	if err != nil {
		t.Fatalf("Alice decrypt: %v", err)
	}
	if string(pt3) != "Hi Alice, I'm fine!" {
		t.Fatalf("msg3 mismatch: got %q", string(pt3))
	}

	// --- Bob sends another message to Alice ---
	ct4, err := bob.RatchetEncrypt([]byte("What's up?"))
	if err != nil {
		t.Fatalf("Bob encrypt 2: %v", err)
	}
	pt4, err := alice.RatchetDecrypt(ct4)
	if err != nil {
		t.Fatalf("Alice decrypt 2: %v", err)
	}
	if string(pt4) != "What's up?" {
		t.Fatalf("msg4 mismatch: got %q", string(pt4))
	}

	// --- Alice sends again (ratchet back) ---
	ct5, err := alice.RatchetEncrypt([]byte("Ratchet back!"))
	if err != nil {
		t.Fatalf("Alice encrypt 3: %v", err)
	}
	pt5, err := bob.RatchetDecrypt(ct5)
	if err != nil {
		t.Fatalf("Bob decrypt 3: %v", err)
	}
	if string(pt5) != "Ratchet back!" {
		t.Fatalf("msg5 mismatch: got %q", string(pt5))
	}
}

// ---------------------------------------------------------------------------
// TestOutOfOrderMessages — messages arrive out of order
// ---------------------------------------------------------------------------

func TestOutOfOrderMessages(t *testing.T) {
	t.Parallel()

	var shared [32]byte
	copy(shared[:], []byte("out-of-order-shared-secret-32b!"))

	alice := NewRatchetState(shared[:], true)
	bob := NewRatchetState(shared[:], false)

	alice.RemotePub = bob.DHPub
	bob.RemotePub = alice.DHPub

	// Alice sends 3 messages
	ct1, _ := alice.RatchetEncrypt([]byte("msg1"))
	ct2, _ := alice.RatchetEncrypt([]byte("msg2"))
	ct3, _ := alice.RatchetEncrypt([]byte("msg3"))

	// Bob receives them out of order: 3, 1, 2

	// Message 3 arrives first — Bob should skip 0,1 and cache keys
	pt3, err := bob.RatchetDecrypt(ct3)
	if err != nil {
		t.Fatalf("Bob decrypt msg3 (out of order): %v", err)
	}
	if string(pt3) != "msg3" {
		t.Fatalf("msg3 mismatch: got %q", string(pt3))
	}

	// Message 1 arrives — should use cached skipped key
	pt1, err := bob.RatchetDecrypt(ct1)
	if err != nil {
		t.Fatalf("Bob decrypt msg1 (out of order): %v", err)
	}
	if string(pt1) != "msg1" {
		t.Fatalf("msg1 mismatch: got %q", string(pt1))
	}

	// Message 2 arrives — should use cached skipped key
	pt2, err := bob.RatchetDecrypt(ct2)
	if err != nil {
		t.Fatalf("Bob decrypt msg2 (out of order): %v", err)
	}
	if string(pt2) != "msg2" {
		t.Fatalf("msg2 mismatch: got %q", string(pt2))
	}
}

// ---------------------------------------------------------------------------
// TestSkippedKeyCleanup — verifies that old skipped keys are evicted
// ---------------------------------------------------------------------------

func TestSkippedKeyCleanup(t *testing.T) {
	t.Parallel()

	var shared [32]byte
	copy(shared[:], []byte("cleanup-test-shared-secret-32b!!"))

	alice := NewRatchetState(shared[:], true)
	bob := NewRatchetState(shared[:], false)

	alice.RemotePub = bob.DHPub
	bob.RemotePub = alice.DHPub

	// Encrypt more than MaxSkippedKeys messages from Alice
	sentCount := MaxSkippedKeys + 5
	ciphertexts := make([][]byte, sentCount)
	for i := 0; i < sentCount; i++ {
		msg := []byte("msg-many")
		ct, err := alice.RatchetEncrypt(msg)
		if err != nil {
			t.Fatalf("encrypt msg %d: %v", i, err)
		}
		ciphertexts[i] = ct
	}

	// Bob decrypts only the last message — all earlier ones are skipped
	lastIdx := sentCount - 1
	pt, err := bob.RatchetDecrypt(ciphertexts[lastIdx])
	if err != nil {
		t.Fatalf("Bob decrypt last msg: %v", err)
	}
	if string(pt) != "msg-many" {
		t.Fatalf("last msg mismatch: got %q", string(pt))
	}

	// Skipped keys map should not exceed MaxSkippedKeys
	bob.mu.Lock()
	skipped := len(bob.SkippedKeys)
	bob.mu.Unlock()

	if skipped > MaxSkippedKeys {
		t.Fatalf("skipped keys count %d exceeds max %d", skipped, MaxSkippedKeys)
	}

	// The skipped keys should be > 0 (we skipped many messages)
	if skipped == 0 {
		t.Fatal("expected some skipped keys to be cached")
	}
}
