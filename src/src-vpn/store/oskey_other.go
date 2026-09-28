//go:build !windows

package store

import "fmt"

func protectKey(raw []byte) ([]byte, error) {
	if len(raw) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}
	return append([]byte("RAW1"), raw...), nil
}

func unprotectKey(blob []byte) ([]byte, error) {
	if len(blob) >= 4 && (string(blob[:4]) == "RAW1" || string(blob[:4]) == "DPAP") {
		blob = blob[4:]
	}
	if len(blob) != 32 {
		return nil, fmt.Errorf("key blob len %d", len(blob))
	}
	return append([]byte(nil), blob...), nil
}
