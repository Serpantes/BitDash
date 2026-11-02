package delivery

import (
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter caps each client at rps requests per second with bursts up to
// burst (a token bucket per client IP), so one visitor cannot turn the server
// into a free BitJita proxy or hold the shared upstream budget.
type RateLimiter struct {
	rps       float64
	burst     float64
	now       func() time.Time
	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	seen   time.Time
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{rps: rps, burst: float64(burst), now: time.Now, buckets: map[string]*bucket{}}
}

// Allow takes one token from the client's bucket.
func (l *RateLimiter) Allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) > time.Minute {
		// A bucket that refilled completely is the same as no bucket.
		for k, b := range l.buckets {
			if now.Sub(b.seen).Seconds()*l.rps >= l.burst {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}
	b, ok := l.buckets[client]
	if !ok {
		b = &bucket{tokens: l.burst, seen: now}
		l.buckets[client] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.seen).Seconds()*l.rps)
	b.seen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Wrap answers 429 to clients over their limit.
func (l *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(ClientIP(r)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP is the visitor's address. Behind Cloudflare every request comes
// from the tunnel, and Cloudflare sets CF-Connecting-IP to the real client
// (overwriting any value the client sent).
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
