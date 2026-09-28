//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func scheduleInstallDelete(paths []string) {
	if len(paths) == 0 {
		return
	}
	f, err := os.CreateTemp("", "proxi-del-*.cmd")
	if err != nil {
		emit("delete-schedule-fail")
		return
	}
	fmt.Fprint(f, "@echo off\r\n")
	fmt.Fprint(f, "ping -n 4 127.0.0.1 >nul\r\n")
	for _, p := range paths {
		fmt.Fprintf(f, "del /f /q \"%s\"\r\n", p)
	}
	fmt.Fprintf(f, "del /f /q \"%s\"\r\n", f.Name())
	name := f.Name()
	_ = f.Close()
	cmd := exec.Command("cmd.exe", "/c", "start", "", "/min", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		emit("delete-schedule-fail")
		return
	}
	emit("delete-scheduled")
}
