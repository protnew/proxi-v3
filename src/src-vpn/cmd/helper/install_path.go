package main

import (
	"path/filepath"
	"strings"
)

func pathUnderAdminDir(exe string, roots []string) bool {
	exe = strings.ToLower(filepath.Clean(exe))
	for _, root := range roots {
		if root == "" {
			continue
		}
		root = strings.ToLower(filepath.Clean(root))
		if exe == root || strings.HasPrefix(exe, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
