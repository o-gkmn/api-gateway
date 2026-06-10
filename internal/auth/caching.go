package auth

import (
	"api-gateway/internal/reqctx"
	"api-gateway/utils"
	"context"
	"crypto/sha256"
	"sync"
	"time"
	"unsafe"
)

type cacheEntry struct {
	claims   *reqctx.Claims
	exp      time.Time
	cachedAt time.Time
}

type CachingVerifier struct {
	mu       sync.RWMutex
	inner    Verifier
	cache    map[string]cacheEntry
	maxSize  int
	ttl      time.Duration
	interval time.Duration
	now      func() time.Time
	done     chan struct{}
	stopOnce sync.Once
}

func NewCachingVerifier(
	inner Verifier,
	maxSize int,
	ttl time.Duration,
	interval time.Duration,
	now func() time.Time,
) *CachingVerifier {
	if now == nil {
		now = time.Now
	}

	if maxSize <= 0 {
		maxSize = 10
	}

	v := &CachingVerifier{
		inner:   inner,
		cache:   make(map[string]cacheEntry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
		now:     now,
		done:    make(chan struct{}),
	}

	go v.clearCache(interval)

	return v
}

func (v *CachingVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	// Verify is called concurrently from many request goroutines, and the sweep
	// goroutine (evictStale) runs in parallel. Go maps are not safe for concurrent
	// access: any combination of writes with reads or other writes triggers a
	// "concurrent map read and map write" runtime fatal — unrecoverable, kills
	// the process.
	//
	// RWMutex protects v.cache:
	//   - RLock for lookups: multiple readers can hold it simultaneously, which
	//     matters because reads are the hot path (every request).
	//   - Lock for any mutation (insert, delete): exclusive.
	//
	// We deliberately release the lock before calling inner.Verify, then reacquire
	// it to store. RSA verification takes ~27µs; holding the lock across it would
	// serialize all in-flight Verify calls behind one slow verify and erase the
	// cache's whole benefit.

	// We hash the token to avoid storing the full token in memory, which could be large and sensitive.
	// Using unsafe to convert string to []byte slice without memory allocation.
	// Standard conversion []byte(token) forces a heap allocation and memory copy,
	// which incurs a performance penalty and triggers Garbage Collector (GC) on hot paths.
	// Since sha256.Sum256 only reads the bytes and does not mutate the underlying data,
	// this zero-allocation unsafe zero-copy operation is safe and highly optimized for the API Gateway.
	hashBytes := sha256.Sum256(unsafe.Slice(unsafe.StringData(token), len(token)))
	hash := string(hashBytes[:])

	v.mu.RLock()
	entry, ok := v.cache[hash]
	v.mu.RUnlock()

	// If the token is in the cache, check if it's still valid considering both the token's expiry and the cache's TTL.
	if ok {
		cacheUntil := entry.cachedAt.Add(v.ttl)
		effectiveExpiry := utils.MinTime(entry.exp, cacheUntil)
		if !v.now().After(effectiveExpiry) {
			return entry.claims, nil // fresh → fast path
		}
	}

	// If token is not in cache or is stale, verify it using the inner verifier (slow path).
	claims, err := v.inner.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	// Before inserting into the cache, we check if we've reached the max size.
	// If so, we evict entries until there's room for the new one.
	v.mu.Lock()
	if len(v.cache) >= v.maxSize {
		for h := range v.cache {
			delete(v.cache, h)
			if len(v.cache) < v.maxSize {
				break
			}
		}
	}

	v.cache[hash] = cacheEntry{
		claims:   claims,
		exp:      claims.Exp,
		cachedAt: v.now(),
	}
	v.mu.Unlock()

	return claims, nil
}

func (v *CachingVerifier) Stop() {
	v.stopOnce.Do(func() {
		close(v.done)
	})
}

func (v *CachingVerifier) clearCache(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			v.evictStale()
		case <-v.done:
			return
		}
	}
}

func (v *CachingVerifier) evictStale() {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now()

	for hash, entry := range v.cache {
		cacheUntil := entry.cachedAt.Add(v.ttl)
		effectiveExpiry := utils.MinTime(entry.exp, cacheUntil)

		if now.After(effectiveExpiry) {
			delete(v.cache, hash)
		}
	}
}
