package storage

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/unkillable-messenger/vpn/ipfs"
)

// mockClient is a fake IPFS client that stores data in memory.
type mockClient struct {
	mu    sync.RWMutex
	store map[string][]byte // cid -> data
	calls int64             // atomic counter for calls
}

func newMockClient() *mockClient {
	return &mockClient{store: make(map[string][]byte)}
}

func (m *mockClient) UploadFile(filename string, data []byte) (*ipfs.UploadResult, error) {
	atomic.AddInt64(&m.calls, 1)
	cid := fmt.Sprintf("cid-%s", filename)
	m.mu.Lock()
	m.store[cid] = make([]byte, len(data))
	copy(m.store[cid], data)
	m.mu.Unlock()
	return &ipfs.UploadResult{CID: cid, Size: int64(len(data))}, nil
}

func (m *mockClient) DownloadFile(cid string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.store[cid]
	if !ok {
		return nil, fmt.Errorf("CID not found: %s", cid)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// delete removes a CID to simulate data loss.
func (m *mockClient) delete(cid string) {
	m.mu.Lock()
	delete(m.store, cid)
	m.mu.Unlock()
}

func TestStoreAndRetrieve(t *testing.T) {
	t.Parallel()

	clients := []IPFSClient{newMockClient(), newMockClient(), newMockClient()}
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	original := []byte("Distributed erasure-coded file content!")
	result, err := d.StoreFile("testfile.bin", original)
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	if len(result.CIDs) != 5 {
		t.Fatalf("expected 5 CIDs, got %d", len(result.CIDs))
	}
	if result.DataLen != len(original) {
		t.Fatalf("DataLen mismatch: got %d, want %d", result.DataLen, len(original))
	}

	recovered, err := d.RetrieveFile(result.CIDs, result.DataLen)
	if err != nil {
		t.Fatalf("RetrieveFile: %v", err)
	}

	if string(recovered) != string(original) {
		t.Errorf("recovered data mismatch.\ngot:  %q\nwant: %q", recovered, original)
	}
}

func TestRetrieveWithMissingShards(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	clients := []IPFSClient{client} // all shards on one mock
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	original := []byte("File surviving partial data loss!")
	result, err := d.StoreFile("resilient.dat", original)
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	// Delete 2 shard CIDs to simulate loss (can lose up to parity=2).
	cidsCopy := make([]string, len(result.CIDs))
	copy(cidsCopy, result.CIDs)
	cidsCopy[2] = "" // mark as missing
	cidsCopy[4] = "" // mark as missing

	recovered, err := d.RetrieveFile(cidsCopy, result.DataLen)
	if err != nil {
		t.Fatalf("RetrieveFile with 2 missing: %v", err)
	}

	if string(recovered) != string(original) {
		t.Errorf("recovered data mismatch.\ngot:  %q\nwant: %q", recovered, original)
	}
}

func TestRetrieveTooManyMissing_fails(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	clients := []IPFSClient{client}
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	original := []byte("This should fail to recover")
	result, err := d.StoreFile("doomed.dat", original)
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	// Remove 3 shards (more than parity=2).
	cidsCopy := make([]string, len(result.CIDs))
	copy(cidsCopy, result.CIDs)
	cidsCopy[0] = ""
	cidsCopy[1] = ""
	cidsCopy[2] = ""

	_, err = d.RetrieveFile(cidsCopy, result.DataLen)
	if err == nil {
		t.Fatal("expected error when too many shards missing, got nil")
	}
	t.Logf("correct error: %v", err)
}

func TestVerifyFile(t *testing.T) {
	t.Parallel()

	client := newMockClient()
	clients := []IPFSClient{client}
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	result, err := d.StoreFile("verify.bin", []byte("verify me"))
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	// All 5 shards available.
	count, err := d.VerifyFile(result.CIDs)
	if err != nil {
		t.Fatalf("VerifyFile: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 available shards, got %d", count)
	}

	// Simulate loss of 2 shards.
	client.delete(result.CIDs[1])
	client.delete(result.CIDs[3])

	count, err = d.VerifyFile(result.CIDs)
	if err != nil {
		t.Fatalf("VerifyFile after deletion: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 available shards, got %d", count)
	}
}

func TestNewDistributor_NoClients(t *testing.T) {
	t.Parallel()

	_, err := NewDistributor(3, 2, nil)
	if err == nil {
		t.Fatal("expected error with no clients, got nil")
	}
}

func TestStoreAndRetrieve_EmptyData(t *testing.T) {
	t.Parallel()

	clients := []IPFSClient{newMockClient()}
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	result, err := d.StoreFile("empty.bin", []byte{})
	if err != nil {
		t.Fatalf("StoreFile empty: %v", err)
	}

	recovered, err := d.RetrieveFile(result.CIDs, result.DataLen)
	if err != nil {
		t.Fatalf("RetrieveFile empty: %v", err)
	}

	if len(recovered) != 0 {
		t.Errorf("expected empty result, got %d bytes", len(recovered))
	}
}

func TestDistributor_MultipleClients(t *testing.T) {
	t.Parallel()

	// 5 separate mock clients, one per shard.
	clients := []IPFSClient{
		newMockClient(), newMockClient(), newMockClient(),
		newMockClient(), newMockClient(),
	}
	d, err := NewDistributor(3, 2, clients)
	if err != nil {
		t.Fatalf("NewDistributor: %v", err)
	}

	original := []byte("Multi-node distribution test payload!")
	result, err := d.StoreFile("multi.bin", original)
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	recovered, err := d.RetrieveFile(result.CIDs, result.DataLen)
	if err != nil {
		t.Fatalf("RetrieveFile: %v", err)
	}

	if string(recovered) != string(original) {
		t.Errorf("mismatch.\ngot:  %q\nwant: %q", recovered, original)
	}

	if d.DataShards() != 3 {
		t.Errorf("DataShards = %d, want 3", d.DataShards())
	}
	if d.ParityShards() != 2 {
		t.Errorf("ParityShards = %d, want 2", d.ParityShards())
	}
}
