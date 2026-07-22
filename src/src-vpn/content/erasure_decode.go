package content

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// erasureHeaderSize is the 8-byte little-endian length prefix prepended to
// payload before splitting, mirroring storage/erasure.go so that data encoded
// by DecodeFromShards' inverse (EncodeToShards) can be trimmed on decode.
const erasureHeaderSize = 8

// DecodeFromShards reconstructs the original payload from a set of Reed-Solomon
// shards. shards may contain nil entries for missing shards; at least dataShards
// non-nil entries are required. The total shard count must equal
// dataShards + parityShards.
//
// This function mirrors the wire format used by storage.ErasureEncoder.Decode:
// the first 8 bytes of the joined data shards are a little-endian uint64 giving
// the original payload length, which is used to trim Reed-Solomon padding.
//
// Shards produced by storage.ErasureEncoder.Encode are directly decodable here.
func DecodeFromShards(shards [][]byte, dataShards, parityShards int) ([]byte, error) {
	if dataShards < 1 {
		return nil, fmt.Errorf("dataShards must be >= 1, got %d", dataShards)
	}
	if parityShards < 0 {
		return nil, fmt.Errorf("parityShards must be >= 0, got %d", parityShards)
	}
	expected := dataShards + parityShards
	if len(shards) != expected {
		return nil, fmt.Errorf("expected %d shards, got %d", expected, len(shards))
	}

	available := 0
	for _, s := range shards {
		if s != nil {
			available++
		}
	}
	if available < dataShards {
		return nil, fmt.Errorf("need at least %d shards, only %d available", dataShards, available)
	}

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("create reed-solomon encoder: %w", err)
	}

	// Reconstruct missing shards in-place.
	if err := enc.Reconstruct(shards); err != nil {
		return nil, fmt.Errorf("reconstruct: %w", err)
	}

	// Verify integrity of the reconstructed shard set.
	ok, err := enc.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("verification failed: reconstructed shards are inconsistent")
	}

	// Join the data shards back into a single buffer.
	var buf bytes.Buffer
	if err := enc.Join(&buf, shards, len(shards[0])*dataShards); err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}
	joined := buf.Bytes()

	if len(joined) < erasureHeaderSize {
		return nil, fmt.Errorf("recovered data too short to contain length header")
	}
	storedLen := binary.LittleEndian.Uint64(joined[:erasureHeaderSize])
	payload := joined[erasureHeaderSize:]
	if uint64(len(payload)) < storedLen {
		return nil, fmt.Errorf("payload too short: got %d, expected %d", len(payload), storedLen)
	}
	return payload[:storedLen], nil
}

// EncodeToShards is the inverse of DecodeFromShards: it splits data into
// dataShards + parityShards Reed-Solomon shards, prepending an 8-byte length
// header so that DecodeFromShards can trim padding. This is provided for
// symmetry and testing; production upload code uses storage.ErasureEncoder.
func EncodeToShards(data []byte, dataShards, parityShards int) ([][]byte, error) {
	if dataShards < 1 {
		return nil, fmt.Errorf("dataShards must be >= 1, got %d", dataShards)
	}
	if parityShards < 0 {
		return nil, fmt.Errorf("parityShards must be >= 0, got %d", parityShards)
	}
	if parityShards == 0 {
		return nil, fmt.Errorf("parityShards must be >= 1 for erasure coding, got 0")
	}

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("create reed-solomon encoder: %w", err)
	}

	padded := make([]byte, erasureHeaderSize+len(data))
	binary.LittleEndian.PutUint64(padded[:erasureHeaderSize], uint64(len(data)))
	copy(padded[erasureHeaderSize:], data)

	shards, err := enc.Split(padded)
	if err != nil {
		return nil, fmt.Errorf("split into shards: %w", err)
	}
	if err := enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("encode parity: %w", err)
	}
	return shards, nil
}
