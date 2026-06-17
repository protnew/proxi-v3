package content

import (
	"bytes"
	"testing"

	"github.com/unkillable-messenger/vpn/storage"
)

func TestDecodeFromShards_AllPresent(t *testing.T) {
	original := []byte("the quick brown fox jumps over the lazy dog 1234567890")

	shards, err := EncodeToShards(original, 4, 2)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}
	if len(shards) != 6 {
		t.Fatalf("expected 6 shards, got %d", len(shards))
	}

	recovered, err := DecodeFromShards(shards, 4, 2)
	if err != nil {
		t.Fatalf("DecodeFromShards: %v", err)
	}
	if !bytes.Equal(recovered, original) {
		t.Fatalf("recovered data mismatch:\n got %q\nwant %q", recovered, original)
	}
}

func TestDecodeFromShards_MissingShards(t *testing.T) {
	original := make([]byte, 4096)
	for i := range original {
		original[i] = byte(i)
	}

	// 4 data + 2 parity → can tolerate up to 2 missing shards.
	shards, err := EncodeToShards(original, 4, 2)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}

	// Drop shard 0 and shard 5 (one data, one parity).
	missing := make([][]byte, len(shards))
	copy(missing, shards)
	missing[0] = nil
	missing[5] = nil

	recovered, err := DecodeFromShards(missing, 4, 2)
	if err != nil {
		t.Fatalf("DecodeFromShards with 2 missing: %v", err)
	}
	if !bytes.Equal(recovered, original) {
		t.Fatal("recovered data does not match original after losing 2 shards")
	}
}

func TestDecodeFromShards_MissingOnlyParity(t *testing.T) {
	original := []byte("erasure decode handles losing all parity shards")

	shards, err := EncodeToShards(original, 3, 2)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}

	// Drop both parity shards (indices 3 and 4).
	missing := make([][]byte, len(shards))
	copy(missing, shards)
	missing[3] = nil
	missing[4] = nil

	recovered, err := DecodeFromShards(missing, 3, 2)
	if err != nil {
		t.Fatalf("DecodeFromShards with parity missing: %v", err)
	}
	if !bytes.Equal(recovered, original) {
		t.Fatal("recovered data mismatch")
	}
}

func TestDecodeFromShards_TooFewShards(t *testing.T) {
	original := []byte("not enough shards to recover")

	shards, err := EncodeToShards(original, 4, 2)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}

	// Drop 3 shards → only 3 available < 4 data shards.
	missing := make([][]byte, len(shards))
	copy(missing, shards)
	missing[0] = nil
	missing[1] = nil
	missing[2] = nil

	_, err = DecodeFromShards(missing, 4, 2)
	if err == nil {
		t.Fatal("expected error when fewer than dataShards are available")
	}
}

func TestDecodeFromShards_WrongCount(t *testing.T) {
	original := []byte("wrong shard count")

	shards, err := EncodeToShards(original, 4, 2)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}

	// Pass a slice with the wrong length.
	_, err = DecodeFromShards(shards[:5], 4, 2)
	if err == nil {
		t.Fatal("expected error for wrong shard count")
	}

	// Mismatched parameters.
	_, err = DecodeFromShards(shards, 3, 2)
	if err == nil {
		t.Fatal("expected error for mismatched dataShards/parityShards")
	}
}

func TestDecodeFromShards_InvalidParams(t *testing.T) {
	if _, err := DecodeFromShards(nil, 0, 2); err == nil {
		t.Fatal("expected error for dataShards=0")
	}
	if _, err := DecodeFromShards(nil, 4, -1); err == nil {
		t.Fatal("expected error for parityShards<0")
	}
}

func TestDecodeFromShards_LargePayload(t *testing.T) {
	original := make([]byte, 64*1024)
	for i := range original {
		original[i] = byte(i * 7)
	}

	shards, err := EncodeToShards(original, 6, 3)
	if err != nil {
		t.Fatalf("EncodeToShards: %v", err)
	}

	// Lose 3 of the 9 shards (max tolerance for 6+3).
	missing := make([][]byte, len(shards))
	copy(missing, shards)
	missing[0] = nil
	missing[4] = nil
	missing[8] = nil

	recovered, err := DecodeFromShards(missing, 6, 3)
	if err != nil {
		t.Fatalf("DecodeFromShards with 3 missing: %v", err)
	}
	if !bytes.Equal(recovered, original) {
		t.Fatal("large payload recovered data mismatch")
	}
}

func TestDecodeFromShards_EmptyPayload(t *testing.T) {
	shards, err := EncodeToShards([]byte{}, 4, 2)
	if err != nil {
		t.Fatalf("EncodeToShards empty: %v", err)
	}

	recovered, err := DecodeFromShards(shards, 4, 2)
	if err != nil {
		t.Fatalf("DecodeFromShards empty: %v", err)
	}
	if len(recovered) != 0 {
		t.Errorf("expected empty recovery, got %d bytes", len(recovered))
	}
}

func TestEncodeToShards_NoParity(t *testing.T) {
	// parityShards=0 is rejected because erasure coding needs at least 1 parity.
	if _, err := EncodeToShards([]byte("x"), 4, 0); err == nil {
		t.Fatal("expected error for parityShards=0")
	}
}

func TestDecodeFromShards_CrossCompatibleWithStorageEncoder(t *testing.T) {
	// Verify that shards produced by storage.ErasureEncoder.Encode can be
	// decoded by content.DecodeFromShards (same wire format).
	original := []byte("cross-compatible erasure wire format")

	enc, err := storage.NewEncoder(4, 2)
	if err != nil {
		t.Fatalf("storage.NewEncoder: %v", err)
	}
	shards, err := enc.Encode(original)
	if err != nil {
		t.Fatalf("storage encode: %v", err)
	}

	recovered, err := DecodeFromShards(shards, 4, 2)
	if err != nil {
		t.Fatalf("DecodeFromShards of storage-encoded shards: %v", err)
	}
	if !bytes.Equal(recovered, original) {
		t.Fatal("cross-compat recovered data mismatch")
	}
}
