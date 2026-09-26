//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestServiceGone_OnlyMissingService(t *testing.T) {
	if !serviceGone(windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		t.Fatal("1060 not treated as gone")
	}
	if serviceGone(windows.ERROR_ACCESS_DENIED) {
		t.Fatal("access denied treated as gone")
	}
}
