package mw

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestRateLimit_RejectAfterBurst(t *testing.T) {
	rl := NewRateLimiter(1, 2)

	called := false
	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	send := func() (int, bool) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "127.0.0.1"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Header().Get("Retry-After") != ""
	}

	if c, _ := send(); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
	if c, _ := send(); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
	if c, ok := send(); c != http.StatusTooManyRequests && !ok {
		t.Errorf("got %d, want %d", c, http.StatusTooManyRequests)
	}
	if !called {
		t.Errorf("handler was not called")
	}
}

func TestRateLimit_ConsumeTokensPerIp(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	called := false
	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	send := func(ip string) (int, bool) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = ip
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Header().Get("Retry-After") != ""
	}

	if c, _ := send("127.0.0.1"); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
	if c, _ := send("127.0.0.1"); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
	if c, ok := send("127.0.0.1"); c != http.StatusTooManyRequests || !ok {
		t.Errorf("got %d, want %d", c, http.StatusTooManyRequests)
	}
	if c, _ := send("1.2.3.4"); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
	if !called {
		t.Errorf("handler was not called")
	}
}

func TestRateLimit_ConcurrentExactBurst(t *testing.T) {
	rl := NewRateLimiter(1, 100)
	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const n = 500
	var allowed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "127.0.0.1"
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code == http.StatusOK {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 100 {
		t.Errorf("got %d allowed, want %d", got, 100)
	}
}

func TestRateLimit_KeyedByIpNotPort(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(ip string) int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = ip
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	if c := send("127.0.0.1:8080"); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}

	if c := send("127.0.0.1:9090"); c != http.StatusTooManyRequests {
		t.Errorf("got %d, want %d", c, http.StatusTooManyRequests)
	}
}

func TestRateLimit_Refill(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	rl := NewRateLimiter(1, 2)
	rl.now = clock.Now

	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func() int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "127.0.0.1"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	send()
	send()
	if c := send(); c != http.StatusTooManyRequests {
		t.Errorf("got %d, want %d", c, http.StatusTooManyRequests)
	}

	clock.Advance(time.Second)

	if c := send(); c != http.StatusOK {
		t.Errorf("got %d, want %d", c, http.StatusOK)
	}
}

func TestRateLimit_Cleanup(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	rl := NewRateLimiter(1, 2)
	rl.now = clock.Now

	h := rl.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(ip string) int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = ip
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	send("127.0.0.1")
	if rl.size() != 1 {
		t.Errorf("got %d, want %d", rl.size(), 1)
	}

	clock.Advance(4 * time.Minute)
	rl.evictStale()
	send("127.0.0.2")

	if rl.size() != 1 {
		t.Errorf("got %d, want %d", rl.size(), 1)
	}
}
