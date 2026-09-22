package vpn

// web_push.go — SL-051 production VAPID + subscription store + send.
// Spec: RFC 8292 (VAPID) + RFC 8030 (Web Push).
// Keys: ECDSA P-256, persisted under DATA_DIR/vapid.json (not ephemeral random bytes).

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PushConfig struct {
	Enabled      bool   `json:"enabled"`
	VapidPublic  string `json:"vapidPublic,omitempty"`
	Phase        string `json:"phase"`
	Note         string `json:"note"`
	Subject      string `json:"subject,omitempty"`
	Subscribers  int    `json:"subscribers,omitempty"`
	KeysSource   string `json:"keysSource,omitempty"` // env|file|generated
}

type vapidFile struct {
	PublicB64  string `json:"publicKey"`
	PrivatePEM string `json:"privateKeyPEM"`
	Subject    string `json:"subject"`
	CreatedAt  string `json:"createdAt"`
}

type pushSub struct {
	Endpoint string          `json:"endpoint"`
	Keys     json.RawMessage `json:"keys,omitempty"`
	Raw      json.RawMessage `json:"-"`
	SavedAt  int64           `json:"savedAt"`
}

var (
	pushMu     sync.RWMutex
	pushCfg    = PushConfig{Enabled: false, Phase: "phase2", Note: "loading VAPID…"}
	vapidPub   string
	vapidPriv  *ecdsa.PrivateKey
	vapidSubj  = "mailto:push@indestructible.local"
	keysSource = ""
	pushSubsMu      sync.Mutex
	pushSubs        []pushSub
	pushSubsLoaded  bool // P16: load push_subscriptions.json once, lazily (env ready by then)
)

func init() {
	// Prefer env, else load/generate file keys.
	if err := loadOrCreateVAPID(); err != nil {
		pushCfg.Note = "VAPID init error: " + err.Error()
	}
}

func vapidPath() string {
	return filepath.Join(dataDir(), "vapid.json")
}

func loadOrCreateVAPID() error {
	// 1) ENV
	if pub := os.Getenv("VAPID_PUBLIC_KEY"); pub != "" {
		privPEM := os.Getenv("VAPID_PRIVATE_KEY")
		if privPEM != "" {
			key, err := parseECPrivate(privPEM)
			if err == nil {
				pushMu.Lock()
				vapidPub = pub
				vapidPriv = key
				if s := os.Getenv("VAPID_SUBJECT"); s != "" {
					vapidSubj = s
				}
				keysSource = "env"
				pushCfg = PushConfig{
					Enabled: true, VapidPublic: pub, Phase: "production",
					Subject: vapidSubj, KeysSource: "env",
					Note: "VAPID from environment",
				}
				pushMu.Unlock()
				return nil
			}
		}
	}

	// 2) File
	path := vapidPath()
	if b, err := os.ReadFile(path); err == nil {
		var f vapidFile
		if json.Unmarshal(b, &f) == nil && f.PublicB64 != "" && f.PrivatePEM != "" {
			key, err := parseECPrivate(f.PrivatePEM)
			if err == nil {
				pushMu.Lock()
				vapidPub = f.PublicB64
				vapidPriv = key
				if f.Subject != "" {
					vapidSubj = f.Subject
				}
				keysSource = "file"
				pushCfg = PushConfig{
					Enabled: true, VapidPublic: vapidPub, Phase: "production",
					Subject: vapidSubj, KeysSource: "file",
					Note: "VAPID from " + path,
				}
				pushMu.Unlock()
				return nil
			}
		}
	}

	// 3) Generate production-grade P-256 and persist
	return GenerateAndPersistVAPID()
}

// GenerateAndPersistVAPID creates ECDSA P-256 VAPID keys and saves to DATA_DIR.
func GenerateAndPersistVAPID() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	pubUncompressed := elliptic.Marshal(elliptic.P256(), key.X, key.Y) // 65 bytes 0x04||X||Y
	pubB64 := base64.RawURLEncoding.EncodeToString(pubUncompressed)

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	_ = os.MkdirAll(dataDir(), 0o700)
	f := vapidFile{
		PublicB64:  pubB64,
		PrivatePEM: string(pemBytes),
		Subject:    vapidSubj,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	raw, _ := json.MarshalIndent(f, "", "  ")
	if err := os.WriteFile(vapidPath(), raw, 0o600); err != nil {
		return err
	}

	pushMu.Lock()
	vapidPub = pubB64
	vapidPriv = key
	keysSource = "generated"
	pushCfg = PushConfig{
		Enabled: true, VapidPublic: pubB64, Phase: "production",
		Subject: vapidSubj, KeysSource: "generated",
		Note: "VAPID generated and persisted to " + vapidPath(),
	}
	pushMu.Unlock()
	return nil
}

func parseECPrivate(s string) (*ecdsa.PrivateKey, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "BEGIN") {
		block, _ := pem.Decode([]byte(s))
		if block == nil {
			return nil, fmt.Errorf("invalid PEM")
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	// raw base64 PKCS8 or EC
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	if k, err := x509.ParseECPrivateKey(raw); err == nil {
		return k, nil
	}
	k8, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, err
	}
	ec, ok := k8.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not EC private key")
	}
	return ec, nil
}

// EnsureDevVAPID kept for API compat — now production generate.
func EnsureDevVAPID() (pub string, err error) {
	pushMu.RLock()
	if vapidPub != "" && vapidPriv != nil {
		p := vapidPub
		pushMu.RUnlock()
		return p, nil
	}
	pushMu.RUnlock()
	if err := GenerateAndPersistVAPID(); err != nil {
		return "", err
	}
	return GetPushConfig().VapidPublic, nil
}

func GetPushConfig() PushConfig {
	ensurePushSubsLoaded()
	pushMu.RLock()
	defer pushMu.RUnlock()
	cfg := pushCfg
	pushSubsMu.Lock()
	cfg.Subscribers = len(pushSubs)
	pushSubsMu.Unlock()
	return cfg
}

// HandlePushConfig GET /api/push/config ; POST ?dev=1 forces ensure keys
func HandlePushConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		_, _ = EnsureDevVAPID()
	}
	cfg := GetPushConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// ensurePushSubsLoaded — P16: subscriptions were written to disk but never
// read back, so every restart silently dropped all subscribers.
func ensurePushSubsLoaded() {
	pushSubsMu.Lock()
	defer pushSubsMu.Unlock()
	if pushSubsLoaded {
		return
	}
	pushSubsLoaded = true
	b, err := os.ReadFile(filepath.Join(dataDir(), "push_subscriptions.json"))
	if err != nil {
		return
	}
	var subs []pushSub
	if json.Unmarshal(b, &subs) == nil && len(subs) > 0 {
		pushSubs = subs
	}
}

// HandlePushSubscribe POST /api/push/subscribe
func HandlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	ensurePushSubsLoaded()
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	var sub pushSub
	_ = json.Unmarshal(raw, &sub)
	sub.Raw = raw
	sub.SavedAt = time.Now().Unix()
	if sub.Endpoint == "" {
		// still store raw for debugging
		sub.Endpoint = "unknown"
	}
	// persist to disk
	pushSubsMu.Lock()
	// dedupe by endpoint
	out := make([]pushSub, 0, len(pushSubs)+1)
	for _, s := range pushSubs {
		if s.Endpoint != sub.Endpoint {
			out = append(out, s)
		}
	}
	out = append(out, sub)
	if len(out) > 100 {
		out = out[len(out)-100:]
	}
	pushSubs = out
	n := len(pushSubs)
	// write file
	b, _ := json.MarshalIndent(pushSubs, "", "  ")
	_ = os.MkdirAll(dataDir(), 0o700)
	_ = os.WriteFile(filepath.Join(dataDir(), "push_subscriptions.json"), b, 0o600)
	pushSubsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "stored": n, "phase": "production", "endpoint": sub.Endpoint,
	})
}

// HandlePushSend POST /api/push/send {title,body} — sends to all stored subs (best-effort)
func HandlePushSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	ensurePushSubsLoaded()
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Title == "" {
		body.Title = "Indestructible"
	}
	if body.Body == "" {
		body.Body = "New message"
	}
	payload, _ := json.Marshal(map[string]string{"title": body.Title, "body": body.Body})

	pushSubsMu.Lock()
	subs := append([]pushSub(nil), pushSubs...)
	pushSubsMu.Unlock()

	sent, failed := 0, 0
	var lastErr string
	for _, s := range subs {
		if s.Endpoint == "" || s.Endpoint == "unknown" {
			failed++
			continue
		}
		if err := sendWebPush(s, payload); err != nil {
			failed++
			lastErr = err.Error()
			continue
		}
		sent++
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "sent": sent, "failed": failed, "lastError": lastErr,
		"note": "Full encrypted Web Push (RFC8291) requires browser keys; VAPID auth applied when possible",
	})
}

// sendWebPush attaches VAPID JWT Authorization. Payload unencrypted for local/dev endpoints;
// production browsers need RFC8291 encryption — we still prove VAPID signing path.
func sendWebPush(sub pushSub, payload []byte) error {
	pushMu.RLock()
	key := vapidPriv
	pub := vapidPub
	subj := vapidSubj
	pushMu.RUnlock()
	if key == nil {
		return fmt.Errorf("no vapid private key")
	}
	// audience = origin of endpoint
	aud := sub.Endpoint
	if i := strings.Index(aud[8:], "/"); i >= 0 && strings.HasPrefix(aud, "http") {
		// scheme://host
		if strings.HasPrefix(aud, "https://") {
			rest := aud[len("https://"):]
			if j := strings.Index(rest, "/"); j >= 0 {
				aud = "https://" + rest[:j]
			}
		} else if strings.HasPrefix(aud, "http://") {
			rest := aud[len("http://"):]
			if j := strings.Index(rest, "/"); j >= 0 {
				aud = "http://" + rest[:j]
			}
		}
	}
	jwt, err := makeVAPIDJWT(key, subj, aud, time.Now().Add(12*time.Hour))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, sub.Endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("TTL", "60")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+pub)
	req.Header.Set("Crypto-Key", "p256ecdsa="+pub)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("push endpoint status %d", resp.StatusCode)
	}
	return nil
}

func makeVAPIDJWT(key *ecdsa.PrivateKey, sub, aud string, exp time.Time) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"aud": aud,
		"exp": exp.Unix(),
		"sub": sub,
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	signingInput := header + "." + payload
	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return "", err
	}
	// R||S fixed 32+32
	sig := make([]byte, 64)
	rb, sb := r.Bytes(), s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VAPIDPublicRaw returns raw public key bytes (for tests).
func VAPIDPublicRaw() string {
	pushMu.RLock()
	defer pushMu.RUnlock()
	return vapidPub
}

// Export for tests — sign check
func SignVAPIDTest(aud string) (jwt string, err error) {
	pushMu.RLock()
	key := vapidPriv
	subj := vapidSubj
	pushMu.RUnlock()
	if key == nil {
		return "", fmt.Errorf("no key")
	}
	return makeVAPIDJWT(key, subj, aud, time.Now().Add(time.Hour))
}

