//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// runDiag writes a zip dump (helper log tail + netsh/route/ipconfig/sc query)
// into %ProgramData%\Proxi\diag\ (fallback: exe dir). BAG-53.
func runDiag() (string, error) {
	dir := filepath.Join(os.Getenv("ProgramData"), "Proxi", "diag")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		if exe, e := os.Executable(); e == nil {
			dir = filepath.Dir(exe)
		} else {
			dir = "."
		}
	}
	name := filepath.Join(dir, fmt.Sprintf("proxi04-diag-%s.zip", time.Now().Format("20060102-150405")))
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	add := func(entry string, data []byte) {
		w, err := zw.Create(entry)
		if err == nil {
			_, _ = w.Write(data)
		}
	}
	addCmd := func(entry string, args ...string) {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			out = append(out, []byte("\nerror: "+err.Error())...)
		}
		add(entry, out)
	}

	if helperLogPath != "" {
		if b, err := os.ReadFile(helperLogPath); err == nil {
			if len(b) > 64*1024 {
				b = b[len(b)-64*1024:]
			}
			add("helper-log-tail.txt", b)
		}
	}
	addCmd("route-print.txt", "route", "print")
	addCmd("ipconfig.txt", "ipconfig", "/all")
	addCmd("sc-query.txt", "sc", "query", helperServiceName)
	addCmd("netsh-wfp-state.txt", "netsh", "wfp", "show", "state")
	emit("diag-written " + name)
	return name, nil
}
