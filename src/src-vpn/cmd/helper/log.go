package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var helperLogPath string

func emit(line string) {
	fmt.Println(line)
	seen := map[string]bool{}
	var paths []string
	if helperLogPath != "" {
		paths = append(paths, helperLogPath)
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "helper-run.log"))
	}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			continue
		}
		fmt.Fprintln(f, line)
		f.Close()
	}
}

func fail(err error) {
	if err == nil {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err.Error())
	emit("ERROR " + err.Error())
	os.Exit(1)
}
