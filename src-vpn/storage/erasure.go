package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// ErasureEncoder splits data into shards using Reed-Solomon erasure coding.
// With K=dataShards and N=dataShards+parityShards, the original data can be
// recovered from any K out of N shards.
type ErasureEncoder struct {
	DataShards   int
	ParityShards int
	enc          reedsolomon.Encoder
}

// NewEncoder creates an ErasureEncoder with the given parameters.
// dataShards must be >= 1, parityShards must be >= 1, and their sum must be <= 256.
func NewEncoder(dataShards, parityShards int) (*ErasureEncoder, error) {
	if dataShards < 1 {
		return nil, fmt.Errorf("dataShards must be >= 1, got %d", dataShards)
	}
	if parityShards < 1 {
		return nil, fmt.Errorf("parityShards must be >= 1, got %d", parityShards)
	}

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("create reed-solomon encoder: %w", err)
	}

	return &ErasureEncoder{
		DataShards:   dataShards,
		ParityShards: parityShards,
		enc:          enc,
	}, nil
}

const dataLenHeaderSize = 8 // uint64 little-endian

// Encode splits data into N = DataShards + ParityShards shards.
// The original data length is prepended as an 8-byte little-endian header
// so that Decode can trim the padding introduced by Reed-Solomon.
func (e *ErasureEncoder) Encode(data []byte) ([][]byte, error) {
	// Prepend original length so we can trim padding on decode.
	padded := make([]byte, dataLenHeaderSize+len(data))
	binary.LittleEndian.PutUint64(padded[:dataLenHeaderSize], uint64(len(data)))
	copy(padded[dataLenHeaderSize:], data)

	shards, err := e.enc.Split(padded)
	if err != nil {
		return nil, fmt.Errorf("split into shards: %w", err)
	}

	if err := e.enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("encode parity: %w", err)
	}

	return shards, nil
}

// Decode reconstructs the original data from a set of shards.
// shards may contain nil entries for missing shards; at least DataShards
// non-nil entries are required. dataLen is the original byte length
// (returned by Encode and stored externally).
func (e *ErasureEncoder) Decode(shards [][]byte, dataLen int) ([]byte, error) {
	if len(shards) != e.DataShards+e.ParityShards {
		return nil, fmt.Errorf("expected %d shards, got %d",
			e.DataShards+e.ParityShards, len(shards))
	}

	// Count available shards.
	available := 0
	for _, s := range shards {
		if s != nil {
			available++
		}
	}
	if available < e.DataShards {
		return nil, fmt.Errorf("need at least %d shards, only %d available",
			e.DataShards, available)
	}

	// Reconstruct missing shards in-place.
	if err := e.enc.Reconstruct(shards); err != nil {
		return nil, fmt.Errorf("reconstruct: %w", err)
	}

	// Verify integrity.
	ok, err := e.enc.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("verification failed: reconstructed shards are inconsistent")
	}

	// Join data shards back into a single byte slice.
	var buf bytes.Buffer
	err = e.enc.Join(&buf, shards, len(shards[0])*e.DataShards)
	recovered := buf.Bytes()
	if err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}

	// Trim padding using the stored length header.
	if len(recovered) < dataLenHeaderSize {
		return nil, fmt.Errorf("recovered data too short to contain length header")
	}
	storedLen := binary.LittleEndian.Uint64(recovered[:dataLenHeaderSize])
	if storedLen != uint64(dataLen) {
		return nil, fmt.Errorf("length mismatch: stored %d, expected %d", storedLen, dataLen)
	}

	payload := recovered[dataLenHeaderSize:]
	if uint64(len(payload)) < storedLen {
		return nil, fmt.Errorf("payload too short: got %d, expected %d", len(payload), storedLen)
	}

	return payload[:storedLen], nil
}

// TotalShards returns the total number of shards (data + parity).
func (e *ErasureEncoder) TotalShards() int {
	return e.DataShards + e.ParityShards
}
