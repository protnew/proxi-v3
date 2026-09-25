//go:build windows

package store

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectKey(raw []byte) ([]byte, error) {
	if len(raw) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}
	in := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var out windows.DataBlob
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	buf := unsafe.Slice(out.Data, out.Size)
	dup := make([]byte, len(buf))
	copy(dup, buf)
	return append([]byte("DPAP"), dup...), nil
}

func unprotectKey(blob []byte) ([]byte, error) {
	if len(blob) >= 4 && string(blob[:4]) == "DPAP" {
		blob = blob[4:]
	}
	if len(blob) == 32 {
		return append([]byte(nil), blob...), nil
	}
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty key blob")
	}
	in := windows.DataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	var out windows.DataBlob
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	buf := unsafe.Slice(out.Data, out.Size)
	dup := make([]byte, len(buf))
	copy(dup, buf)
	return dup, nil
}
