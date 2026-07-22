package tor

import (
	"testing"
)

func TestCreateOnionService_NoTor(t *testing.T) {
	_, err := CreateOnionService("127.0.0.1:19999", 9999)
	if err == nil {
		t.Error("expected error when Tor not available")
	}
	t.Logf("Error (expected): %v", err)
}

func TestOnionService_Address(t *testing.T) {
	s := &OnionService{OnionAddr: "test123abc.onion", Port: 80}
	if s.OnionAddr != "test123abc.onion" {
		t.Errorf("expected test123abc.onion, got %s", s.OnionAddr)
	}
}
