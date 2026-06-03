package enterprise

import (
	"testing"

	"github.com/unkillable-messenger/vpn/store"
)

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSSO_GetAuthURL(t *testing.T) {
	p := NewOIDCProvider("client123", "secret", "https://auth.example.com")
	url := p.GetAuthURL("https://app.example.com/callback", "state123")
	if url == "" {
		t.Error("URL should not be empty")
	}
	t.Logf("Auth URL: %s", url)
}

func TestSSO_ExchangeCode(t *testing.T) {
	p := NewOIDCProvider("client123", "secret", "https://auth.example.com")
	token, info, err := p.ExchangeCode("code123")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
	if info["email"] == nil {
		t.Error("should have email")
	}
}

func TestSSO_ValidateToken(t *testing.T) {
	p := NewOIDCProvider("client123", "secret", "https://auth.example.com")
	info, err := p.ValidateToken("mock_token_code123")
	if err != nil {
		t.Fatal(err)
	}
	if info["sub"] == nil {
		t.Error("should have sub")
	}
	_, err = p.ValidateToken("")
	if err == nil {
		t.Error("empty token should fail")
	}
}

func TestAuditLog(t *testing.T) {
	db := newTestDB(t)
	logger := NewAuditLogger(db)
	logger.Log("alice", "delete_message", "msg123", map[string]interface{}{"reason": "spam"})
	logger.Log("bob", "login", "session", nil)

	logs, err := logger.GetLogs("alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log for alice, got %d", len(logs))
	}

	allLogs, _ := logger.GetLogs("", 10)
	if len(allLogs) != 2 {
		t.Fatalf("expected 2 total logs, got %d", len(allLogs))
	}
}

func TestRetentionPolicy(t *testing.T) {
	db := newTestDB(t)
	policy := RetentionPolicy{MessageTTL: 30, DeleteOnExpiry: true}
	err := ApplyRetentionPolicy(db, policy)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWhiteLabel(t *testing.T) {
	db := newTestDB(t)
	config := &BrandConfig{
		AppName:      "MyChat",
		LogoURL:      "https://example.com/logo.png",
		PrimaryColor: "#FF0000",
		AccentColor:  "#00FF00",
	}
	if err := SetBrandConfig(db, *config); err != nil {
		t.Fatal(err)
	}
	got, err := GetBrandConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "MyChat" {
		t.Errorf("expected MyChat, got %s", got.AppName)
	}
	if got.PrimaryColor != "#FF0000" {
		t.Errorf("expected #FF0000, got %s", got.PrimaryColor)
	}
}
