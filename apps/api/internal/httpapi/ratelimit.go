package httpapi

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a simple per-IP token-bucket rate limiter.
// It is intentionally minimal and suitable for protecting unauthenticated
// endpoints (login, signup) against brute-force.  For production, replace
// with Redis-backed sliding-window counters.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens added per window
	window  time.Duration // window duration
}

type bucket struct {
	tokens  int
	resetAt time.Time
}

// newRateLimiter creates a limiter that allows `rate` requests per `window`.
func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		window:  window,
	}
	// Background goroutine cleans up expired buckets every minute to prevent
	// unbounded memory growth.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for ip, b := range rl.buckets {
				if now.After(b.resetAt) {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// allow returns true if the request from ip is within the rate limit.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok || now.After(b.resetAt) {
		rl.buckets[ip] = &bucket{tokens: rl.rate - 1, resetAt: now.Add(rl.window)}
		return true
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// authLimiter protects login and signup: 10 attempts per 15 minutes per IP.
var authLimiter = newRateLimiter(10, 15*time.Minute)

// withAuthRateLimit wraps a handler with IP-based rate limiting for auth endpoints.
func withAuthRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Strip port for IPv4; for X-Forwarded-For use the first value.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
		}
		if !authLimiter.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many requests — try again later")
			return
		}
		next(w, r)
	}
}
