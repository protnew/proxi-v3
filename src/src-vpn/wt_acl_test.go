package vpn

import "testing"

func TestF3_MetadataDenied(t *testing.T) {
	if !targetDenied("169.254.169.254:80") {
		t.Fatal("metadata must be denied")
	}
	if !targetDenied("127.0.0.1:443") || !targetDenied("10.0.0.5:80") || !targetDenied("localhost:80") {
		t.Fatal("loopback/LAN must be denied")
	}
	if targetDenied("1.1.1.1:443") {
		t.Fatal("public IP must be allowed")
	}
}

func TestF3_StreamBudget(t *testing.T) {
	var b streamBudget
	for i := 0; i < wtMaxStreamsPerSession; i++ {
		if !b.take() {
			t.Fatalf("stream %d should be allowed", i)
		}
	}
	if b.take() {
		t.Fatal("over limit must refuse")
	}
}
