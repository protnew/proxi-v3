package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// StorageProvider abstracts file storage operations, allowing
// easy swapping between Local, IPFS, S3, etc.
type StorageProvider interface {
	SaveFile(filename string, r io.Reader) (string, int64, error)
	GetFile(filename string) (io.ReadCloser, error)
	DeleteFile(filename string) error
}

// LocalStore implements StorageProvider using the local filesystem.
type LocalStore struct {
	BaseDir string
}

// NewLocalStore creates a new LocalStore, ensuring the directory exists.
func NewLocalStore(baseDir string) (*LocalStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalStore{BaseDir: baseDir}, nil
}

func (s *LocalStore) SaveFile(filename string, r io.Reader) (string, int64, error) {
	dstPath := filepath.Join(s.BaseDir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", 0, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, r)
	if err != nil {
		return "", 0, err
	}
	return filename, written, nil
}

func (s *LocalStore) GetFile(filename string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.BaseDir, filename))
}

func (s *LocalStore) DeleteFile(filename string) error {
	return os.Remove(filepath.Join(s.BaseDir, filename))
}

// IPFSStore is a mock implementation of an IPFS-backed storage provider.
type IPFSStore struct {
	APIEndpoint string
}

func NewIPFSStore(endpoint string) *IPFSStore {
	return &IPFSStore{APIEndpoint: endpoint}
}

func (s *IPFSStore) SaveFile(filename string, r io.Reader) (string, int64, error) {
	// Mock implementation
	return "ipfs://QmMockHash1234567890", 0, nil
}

func (s *IPFSStore) GetFile(filename string) (io.ReadCloser, error) {
	// Mock implementation
	return nil, fmt.Errorf("IPFS get not implemented in mock")
}

func (s *IPFSStore) DeleteFile(filename string) error {
	return nil
}
