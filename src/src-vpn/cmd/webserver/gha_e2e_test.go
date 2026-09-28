package main

// GHA-only two-process invite/connect test (TZ-04 §3.2).
// Runs ONLY when PROXI_GHA_E2E=1 — skipped everywhere else.
// Spawns two real core binaries with separate DATA_DIR/PORT/JWT and drives
// the P-G flow over real HTTP RPC: create_invite + start_egress_listener on
// the donor, schnorr-signed connect_invite on the client.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/golang-jwt/jwt/v5"
)

const ghaJWTSecret = "gha-e2e-shared-secret"

type ghaCore struct {
	cmd     *exec.Cmd
	dataDir string
	baseURL string
}

func xOnlyHex(priv *btcec.PrivateKey) string {
	return hex.EncodeToString(priv.PubKey().SerializeCompressed()[1:])
}

func mintAdminJWT(npubHex string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": "gha-" + npubHex[:8],
		"npub":    npubHex,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(ghaJWTSecret))
}

func rpcCall(t *testing.T, c *ghaCore, jwtTok, method string, params map[string]any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	req, _ := http.NewRequest("POST", c.baseURL+"/api/vpn/rpc", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwtTok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s rpc %s: %v", c.baseURL, method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s rpc %s bad json: %s", c.baseURL, method, raw)
	}
	if out.Error != nil {
		t.Fatalf("%s rpc %s error: %s", c.baseURL, method, out.Error.Message)
	}
	return out.Result
}

func startCore(t *testing.T, bin, dataDir, port, adminNpub string) *ghaCore {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"DATA_DIR="+dataDir,
		"PORT="+port,
		"PROXI_BIND=127.0.0.1",
		"JWT_SECRET_PIN=1",
		"JWT_SECRET="+ghaJWTSecret,
		"PROXI_ADMIN_NPUBS="+adminNpub,
		"SENTRY_DSN=",
	)
	cmd.Dir = dataDir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &ghaCore{cmd: cmd, dataDir: dataDir, baseURL: "http://127.0.0.1:" + port}
	// Wait for atomic endpoint file + TCP up.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("core did not come up")
		}
		ep, err := os.ReadFile(filepath.Join(dataDir, "core-endpoint.json"))
		if err == nil {
			var m map[string]any
			if json.Unmarshal(ep, &m) == nil && m["endpoint"] != nil {
				c.baseURL = "http://" + fmt.Sprint(m["endpoint"])
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return c
}

func TestGHA_TwoProcessInvite(t *testing.T) {
	if os.Getenv("PROXI_GHA_E2E") == "" {
		t.Skip("GHA e2e only — set PROXI_GHA_E2E=1")
	}
	bin := os.Getenv("CORE_BIN")
	if bin == "" {
		t.Skip("CORE_BIN not set")
	}
	root := t.TempDir()
	keyA, _ := btcec.NewPrivateKey()
	keyB, _ := btcec.NewPrivateKey()
	npubA, npubB := xOnlyHex(keyA), xOnlyHex(keyB)
	jwtA, _ := mintAdminJWT(npubA)
	jwtB, _ := mintAdminJWT(npubB)

	donor := startCore(t, bin, filepath.Join(root, "donor"), "0", npubA)
	defer donor.cmd.Process.Kill()
	client := startCore(t, bin, filepath.Join(root, "client"), "0", npubB)
	defer client.cmd.Process.Kill()

	// Donor: issue token bound to client npub + open egress on loopback WT.
	inv := rpcCall(t, donor, jwtA, "create_invite", map[string]any{"to": npubB})
	token := inv["token"].(string)
	exp := int64(inv["exp"].(float64))

	eg := rpcCall(t, donor, jwtA, "start_egress_listener", map[string]any{"public_bind": "127.0.0.1:0"})
	wtAddr, _ := eg["wtAddr"].(string)
	certHash, _ := eg["certHash"].(string)
	if wtAddr == "" || certHash == "" {
		t.Fatalf("egress info incomplete: %v", eg)
	}

	// Client: schnorr-sign sha256(token|npubB) and connect.
	sum := sha256.Sum256([]byte(token + "|" + npubB))
	sig, err := schnorr.Sign(keyB, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	res := rpcCall(t, client, jwtB, "connect_invite", map[string]any{
		"wtAddr": wtAddr, "certHash": certHash,
		"token": token, "npub": npubB,
		"sig": hex.EncodeToString(sig.Serialize()), "exp": exp,
	})
	if res["state"] != "accepted" {
		t.Fatalf("connect_invite not accepted: %v", res)
	}
	st := rpcCall(t, client, jwtB, "get_status", nil)
	if st["state"] != "connected" && st["state"] != "connecting" {
		t.Fatalf("client state=%v", st)
	}

	// Spent-token replay path is covered by TestEgress_ReplayTokenSpent.
}
