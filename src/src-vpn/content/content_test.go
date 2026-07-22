package content

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
)

// ---------- helpers ----------

// randBytes returns n pseudo-random bytes using a simple xorshift64 PRNG.
// The output avoids repeating patterns so that chunks get unique hashes.
func randBytes(n int) []byte {
	b := make([]byte, n)
	var state uint64 = 0x123456789ABCDEF0
	for i := range b {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		b[i] = byte(state)
	}
	return b
}

// ---------- chunk.go ----------

func TestChunkFile_Basic(t *testing.T) {
	data := randBytes(2*DefaultChunkSize + 512) // 2.5 MB
	chunks, err := ChunkFile(bytes.NewReader(data), DefaultChunkSize)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	// First two chunks should be full size.
	if len(chunks[0].Data) != DefaultChunkSize {
		t.Errorf("chunk 0 size = %d, want %d", len(chunks[0].Data), DefaultChunkSize)
	}
	if len(chunks[1].Data) != DefaultChunkSize {
		t.Errorf("chunk 1 size = %d, want %d", len(chunks[1].Data), DefaultChunkSize)
	}
	// Last chunk is 512 bytes.
	if len(chunks[2].Data) != 512 {
		t.Errorf("chunk 2 size = %d, want 512", len(chunks[2].Data))
	}
	// Check indices.
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has Index=%d", i, c.Index)
		}
		if c.ID == "" {
			t.Errorf("chunk %d has empty ID", i)
		}
		if c.Hash == [32]byte{} {
			t.Errorf("chunk %d has zero Hash", i)
		}
	}
}

func TestChunkFile_SingleByte(t *testing.T) {
	chunks, err := ChunkFile(bytes.NewReader([]byte{0xAB}), DefaultChunkSize)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Data[0] != 0xAB {
		t.Error("data mismatch")
	}
}

func TestChunkFile_Empty(t *testing.T) {
	_, err := ChunkFile(bytes.NewReader([]byte{}), DefaultChunkSize)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
}

func TestChunkFile_InvalidSize(t *testing.T) {
	_, err := ChunkFile(bytes.NewReader([]byte{1}), 0)
	if err == nil {
		t.Fatal("expected error for chunkSize=0")
	}
}

func TestAssembleChunks_Roundtrip(t *testing.T) {
	original := randBytes(3*DefaultChunkSize + 777)
	chunks, err := ChunkFile(bytes.NewReader(original), DefaultChunkSize)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	reassembled, err := AssembleChunks(chunks)
	if err != nil {
		t.Fatalf("AssembleChunks: %v", err)
	}
	if !bytes.Equal(original, reassembled) {
		t.Fatal("reassembled data does not match original")
	}
}

func TestAssembleChunks_TamperedHash(t *testing.T) {
	data := randBytes(100)
	chunks, err := ChunkFile(bytes.NewReader(data), DefaultChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	// Tamper with data but keep old hash.
	chunks[0].Data[0] ^= 0xFF
	_, err = AssembleChunks(chunks)
	if err == nil {
		t.Fatal("expected error for tampered chunk")
	}
}

func TestAssembleChunks_Empty(t *testing.T) {
	_, err := AssembleChunks(nil)
	if err == nil {
		t.Fatal("expected error for empty chunks")
	}
}

// ---------- encrypt.go ----------

func TestGenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}
	// Generate another one and ensure they differ.
	key2, _ := GenerateMasterKey()
	if key == key2 {
		t.Fatal("two generated keys should not be equal")
	}
}

func TestEncryptDecryptChunk(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	original := Chunk{
		Index: 42,
		Data:  randBytes(1024),
	}
	original.computeHash()

	enc, err := EncryptChunk(key, original)
	if err != nil {
		t.Fatalf("EncryptChunk: %v", err)
	}
	if enc.ID != original.ID {
		t.Error("encrypted chunk ID mismatch")
	}
	if enc.Index != original.Index {
		t.Error("encrypted chunk Index mismatch")
	}

	dec, err := DecryptChunk(key, enc)
	if err != nil {
		t.Fatalf("DecryptChunk: %v", err)
	}
	if dec.Index != original.Index {
		t.Error("decrypted Index mismatch")
	}
	if !bytes.Equal(dec.Data, original.Data) {
		t.Error("decrypted data mismatch")
	}
	if dec.ID != original.ID {
		t.Error("decrypted ID mismatch")
	}
}

func TestEncryptDecryptChunk_WrongKey(t *testing.T) {
	key1, _ := GenerateMasterKey()
	key2, _ := GenerateMasterKey()

	chunk := Chunk{Index: 0, Data: randBytes(256)}
	chunk.computeHash()

	enc, err := EncryptChunk(key1, chunk)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptChunk(key2, enc)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptDecryptManifest(t *testing.T) {
	key, _ := GenerateMasterKey()
	plain := []byte(`{"hello":"world","number":42}`)

	ct, err := EncryptManifest(key, plain)
	if err != nil {
		t.Fatalf("EncryptManifest: %v", err)
	}
	if bytes.Equal(ct, plain) {
		t.Error("ciphertext should not equal plaintext")
	}

	pt, err := DecryptManifest(key, ct)
	if err != nil {
		t.Fatalf("DecryptManifest: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Error("decrypted manifest data mismatch")
	}
}

func TestEncryptDecryptManifest_WrongKey(t *testing.T) {
	key1, _ := GenerateMasterKey()
	key2, _ := GenerateMasterKey()

	ct, _ := EncryptManifest(key1, []byte("secret"))
	_, err := DecryptManifest(key2, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

// ---------- manifest.go ----------

func TestCreateManifest(t *testing.T) {
	data := randBytes(DefaultChunkSize + 512)
	chunks, err := ChunkFile(bytes.NewReader(data), DefaultChunkSize)
	if err != nil {
		t.Fatal(err)
	}

	key, _ := GenerateMasterKey()
	var encChunks []EncryptedChunk
	for _, c := range chunks {
		ec, err := EncryptChunk(key, c)
		if err != nil {
			t.Fatal(err)
		}
		encChunks = append(encChunks, ec)
	}

	m, err := CreateManifest("test-id-001", "video.mp4", int64(len(data)),
		DefaultChunkSize, chunks, encChunks, 4, 6, nil)
	if err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}
	if m.TotalChunks != 2 {
		t.Errorf("TotalChunks = %d, want 2", m.TotalChunks)
	}
	if len(m.Chunks) != 2 {
		t.Errorf("len(Chunks) = %d, want 2", len(m.Chunks))
	}
	if m.FileName != "video.mp4" {
		t.Errorf("FileName = %q", m.FileName)
	}
}

func TestCreateManifest_CountMismatch(t *testing.T) {
	chunks := []Chunk{{Index: 0, Data: []byte{1}}}
	chunks[0].computeHash()
	_, err := CreateManifest("id", "f", 1, 1024, chunks, nil, 0, 0, nil)
	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestCreateManifest_EmptyChunks(t *testing.T) {
	_, err := CreateManifest("id", "f", 0, 1024, nil, nil, 0, 0, nil)
	if err == nil {
		t.Fatal("expected error for empty chunks")
	}
}

func TestVerifyManifest_OK(t *testing.T) {
	m := makeValidManifest(t)
	if err := m.VerifyManifest(); err != nil {
		t.Fatalf("VerifyManifest: %v", err)
	}
}

func TestVerifyManifest_EmptyID(t *testing.T) {
	m := makeValidManifest(t)
	m.ID = ""
	if err := m.VerifyManifest(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestVerifyManifest_EmptyFileName(t *testing.T) {
	m := makeValidManifest(t)
	m.FileName = ""
	if err := m.VerifyManifest(); err == nil {
		t.Fatal("expected error for empty FileName")
	}
}

func TestVerifyManifest_ChunkCountMismatch(t *testing.T) {
	m := makeValidManifest(t)
	m.TotalChunks = 99
	if err := m.VerifyManifest(); err == nil {
		t.Fatal("expected error for chunk count mismatch")
	}
}

func TestVerifyManifest_DuplicateHash(t *testing.T) {
	m := makeValidManifest(t)
	m.Chunks = append(m.Chunks, m.Chunks[0]) // duplicate
	m.TotalChunks = len(m.Chunks)
	if err := m.VerifyManifest(); err == nil {
		t.Fatal("expected error for duplicate hash")
	}
}

func TestVerifyManifest_EmptyHash(t *testing.T) {
	m := makeValidManifest(t)
	m.Chunks[0].Hash = ""
	if err := m.VerifyManifest(); err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	m := makeValidManifest(t)

	data, err := m.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	m2, err := DeserializeManifest(data)
	if err != nil {
		t.Fatalf("DeserializeManifest: %v", err)
	}
	if m2.ID != m.ID {
		t.Errorf("ID mismatch: %q vs %q", m2.ID, m.ID)
	}
	if m2.FileName != m.FileName {
		t.Errorf("FileName mismatch")
	}
	if m2.FileSize != m.FileSize {
		t.Errorf("FileSize mismatch")
	}
	if m2.TotalChunks != m.TotalChunks {
		t.Errorf("TotalChunks mismatch")
	}
	if len(m2.Chunks) != len(m.Chunks) {
		t.Errorf("Chunks length mismatch")
	}
	if m2.ErasureK != m.ErasureK || m2.ErasureN != m.ErasureN {
		t.Errorf("Erasure params mismatch")
	}
}

func TestSerializeDeserialize_RoundtripVerify(t *testing.T) {
	m := makeValidManifest(t)
	data, _ := m.Serialize()
	m2, err := DeserializeManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := m2.VerifyManifest(); err != nil {
		t.Fatalf("VerifyManifest after roundtrip: %v", err)
	}
}

func TestDeserializeManifest_InvalidJSON(t *testing.T) {
	_, err := DeserializeManifest([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// makeValidManifest creates a syntactically valid manifest for tests.
func makeValidManifest(t *testing.T) *ContentManifest {
	t.Helper()
	data := randBytes(DefaultChunkSize + 100)
	chunks, err := ChunkFile(bytes.NewReader(data), DefaultChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := GenerateMasterKey()
	var encChunks []EncryptedChunk
	for _, c := range chunks {
		ec, err := EncryptChunk(key, c)
		if err != nil {
			t.Fatal(err)
		}
		encChunks = append(encChunks, ec)
	}
	m, err := CreateManifest("man-001", "test.mp4", int64(len(data)),
		DefaultChunkSize, chunks, encChunks, 3, 5, []byte("fake-enc-key"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// ---------- hls.go ----------

func TestSegmentReader_Basic(t *testing.T) {
	// 2 segments worth + a partial third.
	segBytes := SegmentDuration * BytesPerSecond // bytes per segment
	total := segBytes*2 + 12345
	data := randBytes(total)

	sr := NewSegmentReader(bytes.NewReader(data))
	var segs []Segment
	for {
		seg, err := sr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		segs = append(segs, *seg)
	}

	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	// First two segments should be full size.
	if len(segs[0].Data) != segBytes {
		t.Errorf("seg 0 size = %d, want %d", len(segs[0].Data), segBytes)
	}
	if len(segs[1].Data) != segBytes {
		t.Errorf("seg 1 size = %d, want %d", len(segs[1].Data), segBytes)
	}
	if len(segs[2].Data) != 12345 {
		t.Errorf("seg 2 size = %d, want 12345", len(segs[2].Data))
	}
	// Check indices.
	for i, s := range segs {
		if s.Index != i {
			t.Errorf("segment %d has Index=%d", i, s.Index)
		}
	}
	// Check durations are reasonable.
	if segs[0].Duration != float64(SegmentDuration) {
		t.Errorf("seg 0 duration = %v, want %d", segs[0].Duration, SegmentDuration)
	}
}

func TestSegmentReader_Empty(t *testing.T) {
	sr := NewSegmentReader(bytes.NewReader(nil))
	_, err := sr.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestSegmentReader_SingleByte(t *testing.T) {
	sr := NewSegmentReader(bytes.NewReader([]byte{0xFF}))
	seg, err := sr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(seg.Data) != 1 || seg.Data[0] != 0xFF {
		t.Error("data mismatch")
	}
	_, err = sr.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF on second call, got %v", err)
	}
}

func TestGenerateM3U8(t *testing.T) {
	segs := []Segment{
		{Index: 0, Duration: 10.0, Data: make([]byte, 100)},
		{Index: 1, Duration: 10.0, Data: make([]byte, 100)},
		{Index: 2, Duration: 3.5, Data: make([]byte, 35)},
	}
	playlist := GenerateM3U8(segs)

	if !strings.Contains(playlist, "#EXTM3U") {
		t.Error("missing #EXTM3U")
	}
	if !strings.Contains(playlist, "#EXT-X-VERSION:3") {
		t.Error("missing #EXT-X-VERSION:3")
	}
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:10") {
		t.Error("missing #EXT-X-TARGETDURATION:10")
	}
	if !strings.Contains(playlist, "#EXTINF:10.000000,") {
		t.Error("missing #EXTINF for 10s segment")
	}
	if !strings.Contains(playlist, "#EXTINF:3.500000,") {
		t.Error("missing #EXTINF for 3.5s segment")
	}
	if !strings.Contains(playlist, "segment_000000.ts") {
		t.Error("missing segment_000000.ts")
	}
	if !strings.Contains(playlist, "segment_000002.ts") {
		t.Error("missing segment_000002.ts")
	}
	if !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Error("missing #EXT-X-ENDLIST")
	}
}

func TestSegmentReader_FullRoundtrip(t *testing.T) {
	// Full pipeline: chunk → encrypt → create manifest → verify → serialize/deserialize
	key, _ := GenerateMasterKey()
	original := randBytes(2*DefaultChunkSize + 333)

	// Chunk
	chunks, err := ChunkFile(bytes.NewReader(original), DefaultChunkSize)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt all chunks
	var encChunks []EncryptedChunk
	for _, c := range chunks {
		ec, err := EncryptChunk(key, c)
		if err != nil {
			t.Fatal(err)
		}
		encChunks = append(encChunks, ec)
	}

	// Create & verify manifest
	m, err := CreateManifest("roundtrip-001", "movie.mp4", int64(len(original)),
		DefaultChunkSize, chunks, encChunks, 5, 8, []byte("enc-key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.VerifyManifest(); err != nil {
		t.Fatal(err)
	}

	// Serialize / deserialize
	jsonData, err := m.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := DeserializeManifest(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	if err := m2.VerifyManifest(); err != nil {
		t.Fatal(err)
	}

	// Decrypt chunks back
	var decChunks []Chunk
	for _, ec := range encChunks {
		dc, err := DecryptChunk(key, ec)
		if err != nil {
			t.Fatal(err)
		}
		decChunks = append(decChunks, dc)
	}

	// Assemble and compare
	reassembled, err := AssembleChunks(decChunks)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, reassembled) {
		oh := fmt.Sprintf("%x", sha256.Sum256(original))
		rh := fmt.Sprintf("%x", sha256.Sum256(reassembled))
		t.Fatalf("full roundtrip data mismatch: original=%s reassembled=%s", oh, rh)
	}
}
