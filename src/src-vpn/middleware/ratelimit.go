// Package middleware provides HTTP middleware for the Unkillable Messenger.
//
// ratelimit.go implements per-user (or per-IP) rate limiting using a token bucket
// algorithm backed by golang.org/x/time/rate.
//
// Each unique identity (user ID from header, or IP address as fallback) gets its
// own rate.Limiter instance stored in a sync.Map. Limiters that haven't been used
// for a configurable period are garbage-collected to prevent memory leaks.
package middleware

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ==================== Rate Limiter ====================

// userLimiter wraps a rate.Limiter with a last-access timestamp for GC.
type userLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	mu       sync.RWMutex
}

func (ul *userLimiter) setLastSeen(t time.Time) {
	ul.mu.Lock()
	ul.lastSeen = t
	ul.mu.Unlock()
}

func (ul *userLimiter) getLastSeen() time.Time {
	ul.mu.RLock()
	defer ul.mu.RUnlock()
	return ul.lastSeen
}

// RateLimiter provides per-user HTTP rate limiting via token buckets.
type RateLimiter struct {
	limiters sync.Map // map[string]*userLimiter

	rate  float64 // tokens per second
	burst int     // maximum burst size

	mu       sync.Mutex
	stopGC   chan struct{}
	interval time.Duration // GC cleanup interval
	maxAge   time.Duration // max time since last access before cleanup
}

// NewRateLimiter creates a new per-user rate limiter.
//
// Parameters:
//   - r:     sustained rate in requests per second (e.g., 10.0)
//   - burst: maximum burst size (e.g., 20)
//
// Returns a *RateLimiter with automatic garbage collection running in background.
func NewRateLimiter(r float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:     r,
		burst:    burst,
		stopGC:   make(chan struct{}),
		interval: 5 * time.Minute,
		maxAge:   30 * time.Minute,
	}

	// Start background garbage collection
	go rl.gcLoop()

	return rl
}

// Allow checks whether a request from the given user ID is allowed.
// Returns true if the request should proceed, false if rate limited.
func (rl *RateLimiter) Allow(userID string) bool {
	now := time.Now()

	val, _ := rl.limiters.LoadOrStore(userID, &userLimiter{
		limiter:  rate.NewLimiter(rate.Limit(rl.rate), rl.burst),
		lastSeen: now,
	})

	ul := val.(*userLimiter)
	ul.setLastSeen(now)

	return ul.limiter.Allow()
}

// Middleware returns an HTTP middleware that rate-limits requests per user/IP.
//
// It looks for user identity in this order:
//  1. X-User-ID header (set by auth layer)
//  2. X-API-Key header (API key)
//  3. Remote IP address (fallback)
//
// If rate limited, responds with 429 Too Many Requests.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := rl.extractUserID(r)

		if !rl.Allow(userID) {
			log.Printf("⏱️ Rate limited: %s %s (user: %s)", r.Method, r.URL.Path, userID)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"Too many requests. Please try again later."}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractUserID determines the user identity from the request.
// Falls back to IP address if no auth headers are present.
func (rl *RateLimiter) extractUserID(r *http.Request) string {
	// 1. Check X-User-ID header (set by auth middleware)
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return "uid:" + uid
	}

	// 2. Check X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "key:" + apiKey
	}

	// 3. Fallback to IP address
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	return "ip:" + ip
}

// ==================== Garbage Collection ====================

// gcLoop periodically removes stale limiters that haven't been accessed recently.
func (rl *RateLimiter) gcLoop() {
	for {
		rl.mu.Lock()
		interval := rl.interval
		rl.mu.Unlock()

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			rl.cleanup()
		case <-rl.stopGC:
			timer.Stop()
			return
		}
	}
}

// cleanup removes limiters that haven't been used for maxAge duration.
func (rl *RateLimiter) cleanup() {
	now := time.Now()
	rl.mu.Lock()
	maxAge := rl.maxAge
	rl.mu.Unlock()

	var removed int

	rl.limiters.Range(func(key, value interface{}) bool {
		ul := value.(*userLimiter)
		if now.Sub(ul.getLastSeen()) > maxAge {
			rl.limiters.Delete(key)
			removed++
		}
		return true
	})

	if removed > 0 {
		log.Printf("🧹 Rate limiter GC: removed %d stale entries", removed)
	}
}

// Stop halts the background garbage collection goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopGC)
}

// ==================== Stats ====================

// Stats returns current rate limiter statistics.
func (rl *RateLimiter) Stats() map[string]interface{} {
	var count int
	rl.limiters.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"activeLimiters": count,
		"rate":           rl.rate,
		"burst":          rl.burst,
		"maxAge":         rl.maxAge.String(),
	}
}
