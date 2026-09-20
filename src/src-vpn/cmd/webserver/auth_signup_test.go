package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/auth"
	"github.com/unkillable-messenger/vpn/middleware"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
)

// P1 test helpers: real keypair + signed kind:22242 auth event.
func makeAuthEvent(priv *btcec.PrivateKey, challenge string) *nostrAuthEvent {
	pkHex := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	ev := &nostrAuthEvent{
		PubKey:    pkHex,
		CreatedAt: time.Now().Unix(),
		Kind:      authEventKind,
		Tags:      [][]string{{"challenge", challenge}},
		Content:   challenge,
	}
	id, _ := nostrEventID(ev)
	ev.ID = hex.EncodeToString(id[:])
	sig, _ := schnorr.Sign(priv, id[:])
	ev.Sig = hex.EncodeToString(sig.Serialize())
	return ev
}

// newTestIdentity returns privkey + npub (64-hex x-only accepted by npubToXOnlyHex).
func newTestIdentity(t *testing.T) (*btcec.PrivateKey, string) {
	t.Helper()
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return priv, hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
}

func fetchChallenge(t *testing.T, base, npub string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"npub": npub})
	resp, err := http.Post(base+"/api/auth/challenge", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	c, _ := out["challenge"].(string)
	if c == "" {
		t.Fatalf("no challenge for %s: status=%d", npub[:12], resp.StatusCode)
	}
	return c
}

// signupReal performs the full P1 flow: challenge → signed event → token.
func signupReal(t *testing.T, base string) (token, npub string) {
	t.Helper()
	priv, npub := newTestIdentity(t)
	challenge := fetchChallenge(t, base, npub)
	ev := makeAuthEvent(priv, challenge)
	b, _ := json.Marshal(map[string]interface{}{
		"npub": npub, "challenge": challenge, "event": ev,
	})
	resp, err := http.Post(base+"/api/auth/signup", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != 200 || out["access_token"] == "" {
		t.Fatalf("signupReal status=%d body=%v", resp.StatusCode, out)
	}
	return out["access_token"], npub
}

// setupAuthServer wires JWT + public signup + protected messages/identity (AUTH-008/009).
func setupAuthServer(t *testing.T) (*httptest.Server, *auth.AuthService, *store.Store) {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PATH", "")
	uploadDir := filepath.Join(dataDir, "uploads")
	os.MkdirAll(uploadDir, 0700)
	storageProvider, _ = storage.NewLocalStore(dataDir)

	rl := middleware.NewRateLimiter(1000, 5000)
	perUserLimiter = rl
	t.Cleanup(func() { rl.Stop() })

	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	vpnDir := filepath.Join(dataDir, "vpn")
	os.MkdirAll(vpnDir, 0700)
	vpnMgr, err = vpn.NewManager(vpnDir)
	if err != nil {
		t.Fatalf("vpn: %v", err)
	}

	authSvc := auth.NewAuthService("test-secret-key-32bytes-long!!")
	s := &Server{db: db, authService: authSvc}
	s.initHub()
	startTime = time.Now()

	publicApiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(h))
	}
	protectedApiChain := func(h http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(rateLimitMiddleware(authMiddleware(authSvc, h))))
	}

	mux := http.NewServeMux()
	// mirror production AUTH routes (P1: challenge-response)
	mux.HandleFunc("/api/auth/challenge", publicApiChain(s.handleAuthChallenge))
	mux.HandleFunc("/api/auth/signup", publicApiChain(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
			return
		}
		var body struct {
			Npub      string          `json:"npub"`
			Username  string          `json:"username"`
			Challenge string          `json:"challenge"`
			Event     *nostrAuthEvent `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if body.Npub == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "npub required")
			return
		}
		if err := verifyAuthEvent(body.Npub, body.Challenge, body.Event); err != nil {
			writeError(w, http.StatusUnauthorized, "AUTH_FAILED", err.Error())
			return
		}
		userID := body.Npub
		if len(userID) >= 16 {
			userID = userID[:16]
		}
		username := body.Username
		if username == "" {
			username = "user_" + userID
			if len(username) > 12 {
				username = username[:12]
			}
		}
		w.Header().Set("Content-Type", "application/json")
		s.db.DB().Exec("INSERT OR IGNORE INTO users (id, npub, username, created_at) VALUES (?, ?, ?, ?)",
			userID, body.Npub, username, time.Now().Unix())
		accessToken, refreshToken, err := s.authService.GenerateTokenPair(userID, body.Npub)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": accessToken, "refresh_token": refreshToken, "user_id": userID,
		})
	}))
	mux.HandleFunc("/api/messages", protectedApiChain(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			s.handleMessagesGet(w, r)
		case "POST":
			s.handleMessagesPost(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST")
		}
	}))
	mux.HandleFunc("/api/identity", protectedApiChain(s.handleIdentityGet))
	mux.HandleFunc("/api/health", publicApiChain(s.handleHealth))
	mux.HandleFunc("/ws", s.handleWS)

	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		db.Close()
	})
	return srv, authSvc, db
}

func TestSignupTable(t *testing.T) {
	srv, authSvc, _ := setupAuthServer(t)

	// P1: подписанный signup через настоящий ключ
	priv, npub := newTestIdentity(t)
	ch1 := fetchChallenge(t, srv.URL, npub)
	okBody := map[string]interface{}{
		"npub": npub, "username": "alice", "challenge": ch1,
		"event": makeAuthEvent(priv, ch1),
	}
	// второй валидный signup (без username)
	priv2, npub2 := newTestIdentity(t)
	ch2 := fetchChallenge(t, srv.URL, npub2)
	okBody2 := map[string]interface{}{
		"npub": npub2, "challenge": ch2, "event": makeAuthEvent(priv2, ch2),
	}
	// неподписанный — должен отвалиться
	noSig := map[string]interface{}{"npub": strings.Repeat("cd", 32), "username": "mallory"}
	// replay: тот же challenge+event второй раз → 401
	replay := map[string]interface{}{
		"npub": npub, "username": "alice2", "challenge": ch1,
		"event": makeAuthEvent(priv, ch1),
	}

	cases := []struct {
		name       string
		method     string
		body       interface{}
		wantStatus int
		wantToken  bool
	}{
		{"ok_signed", "POST", okBody, 200, true},
		{"ok_signed_no_username", "POST", okBody2, 200, true},
		{"replay_rejected", "POST", replay, 401, false},
		{"unsigned_rejected", "POST", noSig, 401, false},
		{"missing_npub", "POST", map[string]string{"username": "x"}, 400, false},
		{"empty_json", "POST", map[string]string{}, 400, false},
		{"get_not_allowed", "GET", nil, 405, false},
		{"bad_json", "POST", "not-json", 400, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if s, ok := tc.body.(string); ok {
				bodyReader = bytes.NewReader([]byte(s))
			} else if tc.body != nil {
				b, _ := json.Marshal(tc.body)
				bodyReader = bytes.NewReader(b)
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			req, err := http.NewRequest(tc.method, srv.URL+"/api/auth/signup", bodyReader)
			if err != nil {
				t.Fatal(err)
			}
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, tc.wantStatus, buf.String())
			}
			if !tc.wantToken {
				return
			}
			var out map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if out["access_token"] == "" || out["refresh_token"] == "" || out["user_id"] == "" {
				t.Fatalf("missing tokens: %+v", out)
			}
			claims, err := authSvc.ValidateToken(out["access_token"])
			if err != nil {
				t.Fatalf("token invalid: %v", err)
			}
			if claims.UserID != out["user_id"] {
				t.Errorf("claims.UserID=%s want %s", claims.UserID, out["user_id"])
			}
		})
	}
}

func TestProtectedMessagesRequireJWT(t *testing.T) {
	srv, _, _ := setupAuthServer(t)

	// no token → 401
	resp, err := http.Get(srv.URL + "/api/messages")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("GET messages without token status=%d want 401", resp.StatusCode)
	}

	// signup → token → GET 200 (P1: real challenge+signature)
	token, _ := signupReal(t, srv.URL)
	tok := map[string]string{"access_token": token}

	req, _ := http.NewRequest("GET", srv.URL+"/api/messages?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok["access_token"])
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r2.Body)
		t.Fatalf("GET with token status=%d body=%s", r2.StatusCode, buf.String())
	}
}

func TestProtectedIdentityRequireJWT(t *testing.T) {
	srv, _, _ := setupAuthServer(t)
	resp, err := http.Get(srv.URL + "/api/identity")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("identity without token status=%d want 401", resp.StatusCode)
	}
}

func TestTwoSignupsIdentityIsolation(t *testing.T) {
	srv, _, _ := setupAuthServer(t)
	signup := func(username string) string {
		t.Helper()
		token, _ := signupReal(t, srv.URL)
		return token
	}
	getIdentity := func(token string) (int, map[string]interface{}) {
		t.Helper()
		req, _ := http.NewRequest("GET", srv.URL+"/api/identity", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	a := signup("alice")
	st, body := getIdentity(a)
	if st != 200 {
		t.Fatalf("alice identity status=%d", st)
	}
	// P3: nsec/mnemonic never leave the server — npub only
	if body["npub"] == nil || body["npub"] == "" {
		t.Fatal("alice should receive npub")
	}
	if _, ok := body["nsec"]; ok {
		t.Fatal("P3: nsec must never be returned")
	}
	if _, ok := body["mnemonic"]; ok {
		t.Fatal("P3: mnemonic must never be returned")
	}
	b := signup("bob")
	st, body = getIdentity(b)
	if st != 403 {
		t.Fatalf("bob identity status=%d want 403", st)
	}
	if _, ok := body["nsec"]; ok {
		t.Fatal("bob must not receive nsec")
	}
}

func TestWSRequiresTokenWhenAuthEnabled(t *testing.T) {
	srv, _, _ := setupAuthServer(t)
	// httptest + websocket upgrade without token should 401
	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("WS without token status=%d want 401", resp.StatusCode)
	}
}
