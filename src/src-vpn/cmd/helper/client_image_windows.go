//go:build windows

package main

import (
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func clientImageFromConn(c net.Conn) (string, error) {
	fder, ok := c.(interface{ Fd() uintptr })
	if !ok {
		return "", windows.ERROR_INVALID_HANDLE
	}
	var pid uint32
	err := windows.GetNamedPipeClientProcessId(windows.Handle(fder.Fd()), &pid)
	if err != nil {
		return "", err
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func installedCorePath() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Proxi04`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if v, _, e := k.GetStringValue("CorePath"); e == nil && v != "" {
			return v
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "proxi04-core.exe")
}

func rememberCorePath(helperExe string) error {
	core := filepath.Join(filepath.Dir(helperExe), "proxi04-core.exe")
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Proxi04`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("CorePath", core)
}
