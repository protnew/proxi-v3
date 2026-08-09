package main

import "testing"

func TestIsPrivateLANOrigin(t *testing.T) {
	cases := map[string]bool{
		"http://192.168.1.5:8090": true,
		"http://10.0.0.2:8090":    true,
		"http://127.0.0.1:8090":  true,
		"http://localhost:8090":  true,
		"http://8.8.8.8:8090":    false,
		"https://evil.com":       false,
	}
	for o, want := range cases {
		if got := isPrivateLANOrigin(o); got != want {
			t.Fatalf("%s: got %v want %v", o, got, want)
		}
	}
}
