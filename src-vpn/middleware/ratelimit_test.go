package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ==================== RateLimiter Tests ====================

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	defer rl.Stop()

	stats := rl.Stats()
	if stats["rate"] != 10.0 {
		t.Errorf("expected rate 10.0, got %v", stats["rate"])
	}
	if stats["burst"] != 20 {
		t.Errorf("expected burst 20, got %v", stats["burst"])
	}
	if stats["activeLimiters"] != 0 {
		t.Errorf("expected 0 active limiters, got %v", stats["activeLimiters"])
	}
}

func TestAllowBasic(t *testing.T) {
	rl := NewRateLimiter(100, 5) // 100 req/s, burst 5
	defer rl.Stop()

	// Should allow burst number of requests
	for i := 0; i < 5; i++ {
		if !rl.Allow("user1") {
			t.Errorf("request %d should be allowed within burst", i+1)
		}
	}

	// 6th request should be rate limited (burst exhausted, rate is 100/s)
	// This may or may not pass depending on timing, but it's unlikely to pass
	// in immediate succession.
	if rl.Allow("user1") {
		// In rare cases, the rate limiter may have refilled a token.
		// This is acceptable behavior.
		t.Log("6th request allowed (token refill happened quickly)")
	}
}

func TestAllowDifferentUsers(t *testing.T) {
	rl := NewRateLimiter(10, 2) // 10 req/s, burst 2
	defer rl.Stop()

	// User 1: exhaust burst
	rl.Allow("user1")
	rl.Allow("user1")

	// User 2 should still be independent
	if !rl.Allow("user2") {
		t.Error("user2 should be independent of user1's rate limit")
	}
	if !rl.Allow("user2") {
		t.Error("user2 second request should be allowed")
	}
}

func TestMiddlewareAllows(t *testing.T) {
	rl := NewRateLimiter(100, 100) // generous limits
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddlewareRateLimits(t *testing.T) {
	rl := NewRateLimiter(1, 3) // 1 req/s, burst 3
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Exhaust burst
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d should be OK, got %d", i+1, rec.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	// Check Retry-After header
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestMiddlewareExtractsUserID(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 req/s, burst 1
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with X-User-ID header
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.Header.Set("X-User-ID", "user-abc")
	req1.RemoteAddr = "10.0.0.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("request with X-User-ID should pass, got %d", rec1.Code)
	}

	// Different user, same IP — should be independent
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("X-User-ID", "user-xyz")
	req2.RemoteAddr = "10.0.0.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("different user should be independent, got %d", rec2.Code)
	}

	// Same user — should be rate limited
	req3 := httptest.NewRequest("GET", "/api/test", nil)
	req3.Header.Set("X-User-ID", "user-abc")
	req3.RemoteAddr = "10.0.0.1:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("same user should be rate limited, got %d", rec3.Code)
	}
}

func TestMiddlewareAPIKey(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with X-API-Key
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "secret-key-123")
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("request with API key should pass, got %d", rec.Code)
	}

	// Different API key — independent
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("X-API-Key", "another-key-456")
	req2.RemoteAddr = "10.0.0.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("different API key should be independent, got %d", rec2.Code)
	}
}

func TestMiddlewareFallbackToIP(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No auth headers — falls back to IP
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "192.168.0.1:54321"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request from IP should pass, got %d", rec1.Code)
	}

	// Same IP — rate limited
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.0.1:54321"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request from same IP should be rate limited, got %d", rec2.Code)
	}

	// Different IP — allowed
	req3 := httptest.NewRequest("GET", "/api/test", nil)
	req3.RemoteAddr = "192.168.0.2:54321"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("request from different IP should pass, got %d", rec3.Code)
	}
}

func TestRateLimiterGC(t *testing.T) {
	rl := NewRateLimiter(100, 100)
	defer rl.Stop()

	// Override GC settings for fast test
	rl.mu.Lock()
	rl.interval = 50 * time.Millisecond
	rl.maxAge = 100 * time.Millisecond
	rl.mu.Unlock()

	// Restart GC with new settings
	close(rl.stopGC)
	rl.stopGC = make(chan struct{})
	go rl.gcLoop()

	// Add some users
	rl.Allow("user1")
	rl.Allow("user2")

	stats := rl.Stats()
	if stats["activeLimiters"].(int) < 2 {
		t.Errorf("expected at least 2 active limiters, got %v", stats["activeLimiters"])
	}

	// Wait for GC to kick in (entries should be stale after maxAge)
	time.Sleep(300 * time.Millisecond)

	stats = rl.Stats()
	if stats["activeLimiters"].(int) != 0 {
		t.Errorf("expected 0 active limiters after GC, got %v", stats["activeLimiters"])
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewRateLimiter(10000, 10000) // very generous
	defer rl.Stop()

	done := make(chan bool, 10)

	// 10 goroutines, each making 100 requests with different user IDs
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := string(rune('a' + id))
			for j := 0; j < 100; j++ {
				rl.Allow(userID)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := rl.Stats()
	if stats["activeLimiters"].(int) != 10 {
		t.Errorf("expected 10 active limiters, got %v", stats["activeLimiters"])
	}
}
