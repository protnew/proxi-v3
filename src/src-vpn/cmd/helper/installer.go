package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type installManifest struct {
	HelperSHA256 string `json:"helper_sha256"`
	TorSHA256    string `json:"tor_sha256,omitempty"`
	Dest         string `json:"dest"`
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func stageInstall(dest, helperSrc, torSrc string) (installManifest, error) {
	var man installManifest
	man.Dest = dest
	helperDst := filepath.Join(dest, "helper.exe")
	if err := copyFile(helperSrc, helperDst); err != nil {
		return man, err
	}
	sum, err := fileSHA256(helperDst)
	if err != nil {
		return man, err
	}
	man.HelperSHA256 = sum
	if torSrc != "" {
		torDst := filepath.Join(dest, "tor.exe")
		if err := copyFile(torSrc, torDst); err != nil {
			return man, err
		}
		ts, err := fileSHA256(torDst)
		if err != nil {
			return man, err
		}
		man.TorSHA256 = ts
	}
	raw, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return man, err
	}
	if err := os.WriteFile(filepath.Join(dest, "install-manifest.json"), append(raw, '\n'), 0o644); err != nil {
		return man, err
	}
	return man, nil
}

func runInstaller(dest, torSrc string, roots []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if !tokenElevated() {
		return fmt.Errorf("installer requires an elevated token")
	}
	if !pathUnderAdminDir(dest, roots) {
		return fmt.Errorf("install dest must be under Program Files")
	}
	man, err := stageInstall(dest, exe, torSrc)
	if err != nil {
		return err
	}
	if err := installHelperService(filepath.Join(dest, "helper.exe")); err != nil {
		return err
	}
	emit(fmt.Sprintf("installed dest=%s sha256=%s", man.Dest, man.HelperSHA256))
	return nil
}
