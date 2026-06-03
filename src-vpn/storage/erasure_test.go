package storage

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

func defaultEncoder(t *testing.T) *ErasureEncoder {
	t.Helper()
	enc, err := NewEncoder(3, 2) // K=3, N=5
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	return enc
}

func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	enc := defaultEncoder(t)
	original := []byte("Hello, erasure coding world! This is a test payload.")

	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if len(shards) != 5 {
		t.Fatalf("expected 5 shards, got %d", len(shards))
	}

	decoded, err := enc.Decode(shards, len(original))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Errorf("decoded data doesn't match original.\ngot:  %q\nwant: %q", decoded, original)
	}
}

func TestDecodeWithMissingShards(t *testing.T) {
	t.Parallel()

	enc := defaultEncoder(t)
	original := []byte("Data that must survive shard loss!")

	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Test recovery with 0, 1, and 2 missing shards (parity count = 2).
	for missing := 0; missing <= enc.ParityShards; missing++ {
		t.Run(fmt.Sprintf("missing_%d", missing), func(t *testing.T) {
			damaged := make([][]byte, len(shards))
			copy(damaged, shards)

			// Zero out random shards.
			r := rand.New(rand.NewSource(42))
			idxs := r.Perm(len(shards))
			for i := 0; i < missing; i++ {
				damaged[idxs[i]] = nil
			}

			decoded, err := enc.Decode(damaged, len(original))
			if err != nil {
				t.Fatalf("Decode with %d missing: %v", missing, err)
			}
			if !bytes.Equal(decoded, original) {
				t.Errorf("decoded data doesn't match original (missing=%d).\ngot:  %q\nwant: %q",
					missing, decoded, original)
			}
		})
	}
}

func TestDecodeWithTooFewShards_fails(t *testing.T) {
	t.Parallel()

	enc := defaultEncoder(t)
	original := []byte("This should fail")

	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Remove 3 shards (only 2 remain < DataShards=3).
	damaged := make([][]byte, len(shards))
	copy(damaged, shards)
	damaged[0] = nil
	damaged[1] = nil
	damaged[2] = nil

	_, err = enc.Decode(damaged, len(original))
	if err == nil {
		t.Fatal("expected error when too few shards available, got nil")
	}
	t.Logf("correct error: %v", err)
}

func TestEncodeEmptyData(t *testing.T) {
	t.Parallel()

	enc := defaultEncoder(t)
	original := []byte{}

	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("Encode empty: %v", err)
	}

	if len(shards) != 5 {
		t.Fatalf("expected 5 shards, got %d", len(shards))
	}

	decoded, err := enc.Decode(shards, 0)
	if err != nil {
		t.Fatalf("Decode empty: %v", err)
	}

	if len(decoded) != 0 {
		t.Errorf("expected empty decoded, got %d bytes: %q", len(decoded), decoded)
	}
}

func TestEncodeLargeData(t *testing.T) {
	t.Parallel()

	enc := defaultEncoder(t)
	original := make([]byte, 1024*1024) // 1 MB
	r := rand.New(rand.NewSource(123))
	r.Read(original)

	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("Encode large: %v", err)
	}

	// Remove 2 parity shards.
	damaged := make([][]byte, len(shards))
	copy(damaged, shards)
	damaged[3] = nil
	damaged[4] = nil

	decoded, err := enc.Decode(damaged, len(original))
	if err != nil {
		t.Fatalf("Decode large: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Error("decoded large data doesn't match original")
	}
}

func TestNewEncoder_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		data   int
		parity int
		err    bool
	}{
		{0, 2, true},   // data must be >= 1
		{3, 0, true},   // parity must be >= 1
		{-1, 2, true},  // negative data
		{3, -1, true},  // negative parity
		{1, 1, false},  // minimum valid
		{3, 2, false},  // K=3 N=5
		{10, 10, false}, // larger
	}

	for _, tc := range tests {
		name := fmt.Sprintf("data_%d_parity_%d", tc.data, tc.parity)
		t.Run(name, func(t *testing.T) {
			_, err := NewEncoder(tc.data, tc.parity)
			if tc.err && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.err && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
