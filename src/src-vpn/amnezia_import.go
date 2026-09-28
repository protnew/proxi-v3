package vpn

// amnezia_import.go — prepare conf for AmneziaVPN/AmneziaWG GUI import + launch helper.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ImportPrepResult is returned by PrepareAmneziaImport.
type ImportPrepResult struct {
	OK            bool   `json:"ok"`
	ConfPath      string `json:"confPath,omitempty"`
	UserCopyPath  string `json:"userCopyPath,omitempty"`
	HelperPath    string `json:"helperPath,omitempty"`
	AmneziaVPN    string `json:"amneziaVpn,omitempty"`
	Launched      bool   `json:"launched"`
	Note          string `json:"note"`
	Instructions  []string `json:"instructions"`
}

// PrepareAmneziaImport copies conf to user-visible folder and launches AmneziaVPN if present.
func PrepareAmneziaImport() ImportPrepResult {
	res := ImportPrepResult{
		Instructions: []string{
			"1. Откроется AmneziaVPN (если установлен)",
			"2. В AmneziaVPN: + → «Я могу импортировать конфиг файла» / Import",
			"3. Выберите файл indestructible.conf",
			"4. Подключите туннель",
			"Примечание: без реального peer endpoint туннель не выйдет в интернет — conf доказывает kernel path",
		},
	}

	// Prefer last tunnel conf
	st := GetTunnelStatus()
	confPath := st.ConfPath
	if confPath == "" {
		confPath = filepath.Join(amneziaDir(), "indestructible.conf")
	}
	if _, err := os.Stat(confPath); err != nil {
		// generate a starter conf so user always has a file
		cfg := GetAmneziaConfig()
		cfg.Enabled = true
		priv, pub, err := GenerateWGKeyPair()
		if err != nil {
			res.Note = "no conf and keygen failed: " + err.Error()
			return res
		}
		_ = pub
		body := BuildAmneziaWGConfFull(priv, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "203.0.113.10:51820", "0.0.0.0/0, ::/0", "10.66.66.2/32", "", cfg)
		_ = os.MkdirAll(amneziaDir(), 0o700)
		confPath = filepath.Join(amneziaDir(), "indestructible.conf")
		if err := os.WriteFile(confPath, []byte(body), 0o600); err != nil {
			res.Note = "write conf: " + err.Error()
			return res
		}
	}
	res.ConfPath = confPath

	// Copy to Desktop + Documents for easy GUI pick
	home, _ := os.UserHomeDir()
	targets := []string{
		filepath.Join(home, "Desktop", "Indestructible-Amnezia"),
		filepath.Join(home, "Documents", "Indestructible-Amnezia"),
		filepath.Join(dataDir(), "amnezia-import"),
	}
	var userCopy string
	raw, err := os.ReadFile(confPath)
	if err != nil {
		res.Note = "read conf: " + err.Error()
		return res
	}
	for _, dir := range targets {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		dst := filepath.Join(dir, "indestructible.conf")
		if err := os.WriteFile(dst, raw, 0o600); err == nil {
			userCopy = dst
			// helper bat
			bat := filepath.Join(dir, "OPEN-AMNEZIA-IMPORT.bat")
			batBody := buildImportBat(dst)
			_ = os.WriteFile(bat, []byte(batBody), 0o755)
			res.HelperPath = bat
			// readme
			_ = os.WriteFile(filepath.Join(dir, "README.txt"), []byte(strings.Join(res.Instructions, "\r\n")+"\r\n\r\nConf: "+dst+"\r\n"), 0o644)
			break
		}
	}
	res.UserCopyPath = userCopy
	if userCopy == "" {
		res.Note = "conf ready but could not copy to Desktop/Documents"
		res.OK = true
		return res
	}

	// Find AmneziaVPN
	avpn := ""
	if runtime.GOOS == "windows" {
		cands := []string{
			`C:\Program Files\AmneziaVPN\AmneziaVPN.exe`,
			`C:\Program Files (x86)\AmneziaVPN\AmneziaVPN.exe`,
		}
		for _, c := range cands {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				avpn = c
				break
			}
		}
	}
	res.AmneziaVPN = avpn

	// AUTO-LAUNCH DISABLED (2026-09-18): never start AmneziaVPN.exe / explorer / bat.
	// Product VPN is in-app userspace; external GUI is optional manual export only.
	_ = avpn
	res.Launched = false
	if runtime.GOOS == "windows" {
		res.Note = "conf ready for manual export only; AmneziaVPN auto-launch disabled"
	}

	res.OK = true
	res.Note = fmt.Sprintf("conf copied %s · Amnezia launched=%v · %s", userCopy, res.Launched, time.Now().Format(time.RFC3339))
	return res
}

func buildImportBat(confPath string) string {
	return `@echo off
chcp 65001 >nul
echo === Indestructible → AmneziaVPN import ===
echo Conf: ` + confPath + `
echo.
echo 1) Сейчас откроется папка с conf
echo 2) Запустится AmneziaVPN
echo 3) В Amnezia: добавить сервер → импорт из файла → выбери indestructible.conf
echo.
explorer /select,"` + confPath + `"
if exist "C:\Program Files\AmneziaVPN\AmneziaVPN.exe" (
  echo AmneziaVPN auto-start disabled — open manually only if you want
) else (
  echo AmneziaVPN.exe not found
)
echo.
echo WireGuard kernel (optional, needs UAC once):
echo   msiexec /i C:\Hermes\indestructible-run\wireguard-amd64-1.1.msi
echo   then: "C:\Program Files\WireGuard\wireguard.exe" /installtunnelservice "` + confPath + `"
pause
`
}
