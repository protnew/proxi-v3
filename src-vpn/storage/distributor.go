package storage

import (
	"fmt"
	"sync"

	"github.com/unkillable-messenger/vpn/ipfs"
)

// IPFSClient is the interface the Distributor needs from IPFS.
// Extracted from ipfs.Client so tests can inject mocks.
type IPFSClient interface {
	UploadFile(filename string, data []byte) (*ipfs.UploadResult, error)
	DownloadFile(cid string) ([]byte, error)
}

// Distributor handles erasure-coded storage and retrieval across IPFS nodes.
type Distributor struct {
	encoder *ErasureEncoder
	clients []IPFSClient
}

// NewDistributor creates a Distributor that erasure-encodes data with the
// given parameters and distributes shards across the provided IPFS clients.
// At least one IPFS client is required.
func NewDistributor(dataShards, parityShards int, clients []IPFSClient) (*Distributor, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("at least one IPFS client is required")
	}

	enc, err := NewEncoder(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("create encoder: %w", err)
	}

	return &Distributor{
		encoder: enc,
		clients: clients,
	}, nil
}

// StoreResult holds metadata about a stored file.
type StoreResult struct {
	CIDs    []string // CID for each shard (order matters for retrieval)
	DataLen int      // original data length (needed for decode)
}

// StoreFile erasure-encodes data and uploads each shard to a different IPFS node.
// Shards are round-robin assigned to clients. Returns the CIDs of all shards
// and the original data length (needed for Decode).
func (d *Distributor) StoreFile(name string, data []byte) (*StoreResult, error) {
	shards, err := d.encoder.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("erasure encode: %w", err)
	}

	cids := make([]string, len(shards))
	errCh := make(chan error, len(shards))
	var mu sync.Mutex

	for i, shard := range shards {
		go func(idx int, shardData []byte) {
			client := d.clients[idx%len(d.clients)]
			res, err := client.UploadFile(
				fmt.Sprintf("%s.shard.%d", name, idx),
				shardData,
			)
			if err != nil {
				errCh <- fmt.Errorf("upload shard %d: %w", idx, err)
				return
			}
			mu.Lock()
			cids[idx] = res.CID
			mu.Unlock()
			errCh <- nil
		}(i, shard)
	}

	for range shards {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	return &StoreResult{
		CIDs:    cids,
		DataLen: len(data),
	}, nil
}

// RetrieveFile downloads available shards from IPFS and reconstructs the original data.
// cids must contain exactly TotalShards entries; entries can be empty strings
// for missing/unavailable shards. At least DataShards shards must be available.
func (d *Distributor) RetrieveFile(cids []string, dataLen int) ([]byte, error) {
	if len(cids) != d.encoder.TotalShards() {
		return nil, fmt.Errorf("expected %d CIDs, got %d",
			d.encoder.TotalShards(), len(cids))
	}

	shards := make([][]byte, len(cids))
	errCh := make(chan error, len(cids))
	var mu sync.Mutex

	for i, cid := range cids {
		if cid == "" {
			// Missing shard — leave as nil for Reed-Solomon reconstruction.
			errCh <- nil
			continue
		}
		go func(idx int, cid string) {
			client := d.clients[idx%len(d.clients)]
			data, err := client.DownloadFile(cid)
			if err != nil {
				// Shard unavailable — set to nil, RS will try to reconstruct.
				errCh <- nil
				return
			}
			mu.Lock()
			shards[idx] = data
			mu.Unlock()
			errCh <- nil
		}(i, cid)
	}

	for range cids {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	return d.encoder.Decode(shards, dataLen)
}

// VerifyFile checks how many shards are available for download.
// Returns the count of downloadable shards and any error encountered.
func (d *Distributor) VerifyFile(cids []string) (int, error) {
	if len(cids) != d.encoder.TotalShards() {
		return 0, fmt.Errorf("expected %d CIDs, got %d",
			d.encoder.TotalShards(), len(cids))
	}

	available := 0
	var mu sync.Mutex
	errCh := make(chan error, len(cids))

	for i, cid := range cids {
		if cid == "" {
			errCh <- nil
			continue
		}
		go func(idx int, cid string) {
			client := d.clients[idx%len(d.clients)]
			data, err := client.DownloadFile(cid)
			if err == nil && len(data) > 0 {
				mu.Lock()
				available++
				mu.Unlock()
			}
			errCh <- nil
		}(i, cid)
	}

	for range cids {
		if err := <-errCh; err != nil {
			return 0, err
		}
	}

	return available, nil
}

// DataShards returns the number of data shards (K).
func (d *Distributor) DataShards() int { return d.encoder.DataShards }

// ParityShards returns the number of parity shards.
func (d *Distributor) ParityShards() int { return d.encoder.ParityShards }


