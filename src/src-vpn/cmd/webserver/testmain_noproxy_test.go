package main

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// TestMain clears proxy env and forces DefaultTransport to never use a proxy.
// Prevents flaky httptest failures when User/Process HTTP_PROXY points at a dead
// local forwarder (historically 127.0.0.1:8899). See agent-transfer PROXY-ISOLATION.md.
func TestMain(m *testing.M) {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		_ = os.Unsetenv(k)
	}
	_ = os.Setenv("NO_PROXY", "*")
	_ = os.Setenv("no_proxy", "*")

	http.DefaultTransport = &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return nil, nil },
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	os.Exit(m.Run())
}
