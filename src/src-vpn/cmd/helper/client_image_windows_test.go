//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Microsoft/go-winio"
)

func TestClientImage_MatchesDialer(t *testing.T) {
	path := `\\.\pipe\Proxi04VpnHelperTestA3`
	ln, err := winio.ListenPipe(path, &winio.PipeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	errc := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errc <- err
			return
		}
		defer c.Close()
		img, err := clientImageFromConn(c)
		if err != nil {
			errc <- err
			return
		}
		got <- img
	}()
	conn, err := winio.DialPipe(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case err := <-errc:
		t.Fatal(err)
	case img := <-got:
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if !sameExe(img, exe) {
			t.Fatalf("image %s want %s", img, exe)
		}
		if filepath.Base(img) == "" {
			t.Fatal("empty base")
		}
	}
}
