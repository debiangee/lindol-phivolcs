package server

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple per-IP token bucket rate limiter.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     time.Duration // minimum time between requests
	cleanup  time.Duration
}

type visitor struct {
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter that allows 1 request per `rate` duration per IP.
func NewRateLimiter(rate time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		cleanup:  5 * time.Minute,
	}

	// Cleanup stale entries periodically
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{lastSeen: time.Now()}
		return true
	}

	if time.Since(v.lastSeen) < rl.rate {
		return false
	}

	v.lastSeen = time.Now()
	return true
}

// Middleware returns an HTTP middleware that enforces the rate limit.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health and root notices are lightweight and must remain available for monitoring.
		// They should not consume the public API request allowance.
		if r.URL.Path == "/api/health" || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		ip := getClientIP(r)

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Maximum 1 request per minute.","retry_after_sec":60}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
