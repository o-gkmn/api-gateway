package mw

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	refill float64
	last   time.Time
	now    func() time.Time
}

func newTokenBucket(rps, burst float64, now func() time.Time) *tokenBucket {
	return &tokenBucket{tokens: burst, max: burst, refill: rps, last: now(), now: now}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	b.tokens += now.Sub(b.last).Seconds() * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type client struct {
	bucket   *tokenBucket
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*client
	rps      float64
	burst    float64
	now      func() time.Time
	ttl      time.Duration
	done     chan struct{}
	stopOnce sync.Once
}

func NewRateLimiter(rps, burst float64) *RateLimiter {
	r1 := &RateLimiter{
		clients: make(map[string]*client),
		rps:     rps,
		burst:   burst,
		now:     time.Now,
		ttl:     3 * time.Minute,
		done:    make(chan struct{}),
	}
	go r1.cleanup(time.Minute)
	return r1
}

func (rl *RateLimiter) bucketFor(ip string) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	c, ok := rl.clients[ip]
	if !ok {
		c = &client{bucket: newTokenBucket(rl.rps, rl.burst, rl.now)}
		rl.clients[ip] = c
	}
	c.lastSeen = time.Now()
	return c.bucket
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.evictStale()
		case <-rl.done:
			return
		}
	}
}

func (rl *RateLimiter) evictStale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	for ip, c := range rl.clients {
		if now.Sub(c.lastSeen) > rl.ttl {
			delete(rl.clients, ip)
		}
	}
}

func (rl *RateLimiter) size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.clients)
}

func (rl *RateLimiter) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.bucketFor(ip).allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.done)
	})
}
