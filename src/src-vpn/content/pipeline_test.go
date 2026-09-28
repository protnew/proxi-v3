package content

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"

	"github.com/unkillable-messenger/vpn/storage"
)

// pipeRandBytes returns n pseudo-random bytes.
func pipeRandBytes(n int) []byte {
	b := make([]byte, n)
	// Use a simple PRNG for speed (same pattern as existing tests)
	var state uint64 = 0xFEDCBA9876543210
	for i := range b {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		b[i] = byte(state)
	}
	return b
}

// setupPipeline creates a pipeline with encryption key set.
func setupPipeline(chunkSize, k, n int) *UploadPipeline {
	p := NewUploadPipeline(chunkSize, k, n)
	key, _ := GenerateMasterKey()
	p.MasterKey = key
	return p
}

// uploadAndGetEncryptedChunks is a helper that performs upload and extracts
// encrypted chunk data as a map[int][]byte for download.
func uploadAndGetEncryptedChunks(p *UploadPipeline, filename string, data []byte) (*ContentManifest, map[int][]byte, error) {
	manifest, err := p.ProcessUpload(filename, bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	// Re-derive encrypted chunks by encrypting with the same key
	allData := data
	var chunks []Chunk
	if len(allData) > 0 {
		chunks, err = ChunkFile(bytes.NewReader(allData), p.ChunkSize)
		if err != nil {
			return nil, nil, err
		}
	}
	encMap := make(map[int][]byte)
	for i, c := range chunks {
		ec, err := EncryptChunk(p.MasterKey, c)
		if err != nil {
			return nil, nil, err
		}
		encMap[i] = ec.Data
	}
	return manifest, encMap, nil
}

func TestUploadDownloadRoundtrip(t *testing.T) {
	// Upload 5MB file → download → verify identical
	p := setupPipeline(1*1024*1024, 4, 6) // 1MB chunks, 4+2 erasure

	data := pipeRandBytes(5 * 1024 * 1024) // 5 MB
	manifest, encMap, err := uploadAndGetEncryptedChunks(p, "bigfile.bin", data)
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if manifest.TotalChunks != 5 {
		t.Errorf("expected 5 chunks, got %d", manifest.TotalChunks)
	}
	if manifest.FileSize != int64(len(data)) {
		t.Errorf("filesize mismatch: got %d, want %d", manifest.FileSize, len(data))
	}

	// Download
	recovered, err := p.ProcessDownload(manifest, encMap)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	if !bytes.Equal(data, recovered) {
		t.Fatal("recovered data does not match original")
	}
}

func TestUploadDownloadWithMissingShards(t *testing.T) {
	// Use erasure coding: 2 data + 2 parity shards → can lose up to 2
	chunkSize := 1024 // small chunks for testing
	p := setupPipeline(chunkSize, 2, 4)
	encoder, err := storage.NewEncoder(2, 2)
	if err != nil {
		t.Fatalf("create encoder: %v", err)
	}

	data := pipeRandBytes(1500) // will produce 2 chunks
	chunks, err := ChunkFile(bytes.NewReader(data), chunkSize)
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// Encrypt chunks
	encChunks := make([]EncryptedChunk, len(chunks))
	for i, c := range chunks {
		ec, err := EncryptChunk(p.MasterKey, c)
		if err != nil {
			t.Fatalf("encrypt chunk %d: %v", i, err)
		}
		encChunks[i] = ec
	}

	// Create manifest
	manifest, err := CreateManifest("erasure-test", "test.dat", int64(len(data)),
		chunkSize, chunks, encChunks, 2, 4, nil)
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}

	// For each encrypted chunk, erasure encode and simulate losing 2 shards
	finalEncMap := make(map[int][]byte)
	for i, ec := range encChunks {
		shards, err := encoder.Encode(ec.Data)
		if err != nil {
			t.Fatalf("erasure encode chunk %d: %v", i, err)
		}

		// Simulate losing shard 0 and shard 1 (2 missing)
		availableShards := make([][]byte, len(shards))
		availableShards[2] = shards[2] // shard 2 present
		availableShards[3] = shards[3] // shard 3 present
		// shards 0 and 1 are nil (missing)

		// Recover from remaining shards
		recovered, err := encoder.Decode(availableShards, len(ec.Data))
		if err != nil {
			t.Fatalf("erasure decode chunk %d: %v", i, err)
		}

		if !bytes.Equal(recovered, ec.Data) {
			t.Fatalf("erasure recovery for chunk %d produced wrong data", i)
		}
		finalEncMap[i] = recovered
	}

	// Download with recovered chunks
	result, err := p.ProcessDownload(manifest, finalEncMap)
	if err != nil {
		t.Fatalf("download with erasure-recovered chunks: %v", err)
	}

	if !bytes.Equal(data, result) {
		t.Fatal("final recovered data does not match original")
	}
}

func TestUploadDownloadTamperedChunk(t *testing.T) {
	p := setupPipeline(1024, 2, 4)

	data := pipeRandBytes(2048)
	manifest, encMap, err := uploadAndGetEncryptedChunks(p, "tampered.bin", data)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	// Tamper with chunk 0's encrypted data
	tampered := make([]byte, len(encMap[0]))
	copy(tampered, encMap[0])
	tampered[0] ^= 0xFF
	encMap[0] = tampered

	// Download should fail (GCM authentication will fail on tampered data)
	_, err = p.ProcessDownload(manifest, encMap)
	if err == nil {
		t.Fatal("expected error when downloading with tampered chunk")
	}
}

func TestUploadEmptyFile(t *testing.T) {
	p := setupPipeline(1024, 2, 4)

	manifest, err := p.ProcessUpload("empty.bin", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("upload empty file: %v", err)
	}
	if manifest.TotalChunks != 0 {
		t.Errorf("expected 0 chunks for empty file, got %d", manifest.TotalChunks)
	}
	if manifest.FileSize != 0 {
		t.Errorf("expected 0 file size, got %d", manifest.FileSize)
	}
	if len(manifest.Chunks) != 0 {
		t.Errorf("expected empty chunks, got %d", len(manifest.Chunks))
	}

	// Download empty
	result, err := p.ProcessDownload(manifest, map[int][]byte{})
	if err != nil {
		t.Fatalf("download empty: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d bytes", len(result))
	}
}

func TestPipelineEncryption(t *testing.T) {
	// Download with wrong key should fail
	p := setupPipeline(1024, 2, 4)

	data := pipeRandBytes(2048)
	manifest, encMap, err := uploadAndGetEncryptedChunks(p, "encrypted.bin", data)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	// Create a different pipeline with a different key
	wrongP := NewUploadPipeline(1024, 2, 4)
	var wrongKey [32]byte
	_, _ = rand.Read(wrongKey[:])
	wrongP.MasterKey = wrongKey

	// Attempt download with wrong key
	_, err = wrongP.ProcessDownload(manifest, encMap)
	if err == nil {
		t.Fatal("expected error when downloading with wrong key")
	}
}

func TestUploadDownloadRoundtrip_NoErasure(t *testing.T) {
	// Test with K == N (no erasure coding)
	p := setupPipeline(512, 0, 0) // no erasure

	data := pipeRandBytes(1500)
	manifest, encMap, err := uploadAndGetEncryptedChunks(p, "noerasure.bin", data)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	recovered, err := p.ProcessDownload(manifest, encMap)
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	if !bytes.Equal(data, recovered) {
		t.Fatal("no-erasure roundtrip: data mismatch")
	}
}

func TestPipelineConcurrent(t *testing.T) {
	// Concurrent uploads/downloads
	p := setupPipeline(1024, 2, 4)

	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := pipeRandBytes(2048 + idx*100)
			filename := fmt.Sprintf("concurrent_%d.bin", idx)

			manifest, encMap, err := uploadAndGetEncryptedChunks(p, filename, data)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d upload: %w", idx, err)
				return
			}

			recovered, err := p.ProcessDownload(manifest, encMap)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d download: %w", idx, err)
				return
			}

			if !bytes.Equal(data, recovered) {
				errCh <- fmt.Errorf("goroutine %d: data mismatch", idx)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent pipeline: %v", err)
	}
}
