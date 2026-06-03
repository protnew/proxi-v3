package enterprise

import (
	"fmt"
	"net/url"
)

// SSOProvider handles OIDC/SAML authentication.
type SSOProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
}

// NewOIDCProvider creates an OpenID Connect provider.
func NewOIDCProvider(clientID, clientSecret, issuerURL string) *SSOProvider {
	return &SSOProvider{
		Name:         "oidc",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      issuerURL + "/auth",
		TokenURL:     issuerURL + "/token",
	}
}

// GetAuthURL returns the URL to redirect users for authentication.
func (p *SSOProvider) GetAuthURL(redirectURL, state string) string {
	u, _ := url.Parse(p.AuthURL)
	q := u.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", redirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	q.Set("scope", "openid profile email")
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeCode exchanges an auth code for tokens (mock).
func (p *SSOProvider) ExchangeCode(code string) (token string, userInfo map[string]interface{}, err error) {
	// In production: POST to TokenURL with code, client_id, client_secret
	return "mock_token_" + code, map[string]interface{}{
		"sub":   "user123",
		"email": "user@example.com",
		"name":  "Test User",
	}, nil
}

// ValidateToken validates a JWT token (mock).
func (p *SSOProvider) ValidateToken(token string) (map[string]interface{}, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}
	return map[string]interface{}{
		"sub":   "user123",
		"email": "user@example.com",
	}, nil
}
