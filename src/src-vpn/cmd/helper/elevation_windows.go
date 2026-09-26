//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func tokenElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	var elev uint32
	var out uint32
	err = windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elev)), uint32(unsafe.Sizeof(elev)), &out)
	return err == nil && elev != 0
}
