package nat

import (
	"strings"
	"testing"
)

func TestDefaultSTUNServers(t *testing.T) {
	t.Parallel()

	if len(DefaultSTUNServers) == 0 {
		t.Fatal("DefaultSTUNServers is empty")
	}

	for i, srv := range DefaultSTUNServers {
		if srv == "" {
			t.Errorf("server [%d] is empty", i)
			continue
		}
		if !strings.Contains(srv, ":") {
			t.Errorf("server [%d] %q missing port", i, srv)
		}
	}

	t.Logf("STUN servers: %v", DefaultSTUNServers)
}

func TestGetLocalIP(t *testing.T) {
	t.Parallel()

	ip := getLocalIP()
	if ip == "" {
		t.Fatal("getLocalIP returned empty string")
	}
	// Even in restricted environments it should return "0.0.0.0" at minimum.
	t.Logf("getLocalIP = %s", ip)
}

func TestGetTURNConfig(t *testing.T) {
	t.Parallel()

	configs := GetTURNConfig()
	if len(configs) == 0 {
		t.Fatal("GetTURNConfig returned empty slice")
	}

	for i, cfg := range configs {
		urls, ok := cfg["urls"]
		if !ok {
			t.Errorf("config [%d] missing 'urls' key", i)
			continue
		}
		urlList, ok := urls.([]string)
		if !ok {
			t.Errorf("config [%d] 'urls' is not []string", i)
			continue
		}
		if len(urlList) == 0 {
			t.Errorf("config [%d] has empty URL list", i)
		}
		for j, u := range urlList {
			if !strings.HasPrefix(u, "turn:") {
				t.Errorf("config [%d] url [%d] = %q, expected 'turn:' prefix", i, j, u)
			}
		}
	}
}
