package auth

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCachingVerifier_CacheHit(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"
	claims, err := cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 1 {
		t.Errorf("inner verifier not cached: %+v", ic)
	}
}

func TestCachingVerifier_CacheMiss(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	firstToken := "token"
	firstClaims, err := cv.Verify(context.Background(), firstToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firstClaims.Sub != firstToken {
		t.Errorf("unexpected claims: %+v", firstClaims)
	}

	secondToken := "token2"
	secondClaims, err := cv.Verify(context.Background(), secondToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secondClaims.Sub != secondToken {
		t.Errorf("unexpected claims: %+v", secondClaims)
	}

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 2 {
		t.Errorf("expected vCount=2, got %d", ic)
	}
}

func TestCachingVerifier_TokenExpired(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Minute).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Hour, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"
	claims, err := cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	clock.Advance(2 * time.Minute)
	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 2 {
		t.Errorf("expected vCount=2, got %d", ic)
	}
}

func TestCachingVerifier_CacheExpired(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"
	claims, err := cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	clock.Advance(2 * time.Minute)
	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 2 {
		t.Errorf("expected vCount=2, got %d", ic)
	}
}

func TestCachingVerifier_ErrorsWillNotBeCached(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    errors.New("error"),
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"

	_, err := cv.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	_, err = cv.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 2 {
		t.Errorf("expected vCount=2, got %d", ic)
	}
}

func TestCachingVerifier_CapEviction(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 3, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"
	claims, err := cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	token = "token2"
	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	token = "token3"
	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	token = "token4"
	claims, err = cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	if len(cv.cache) != 3 {
		t.Errorf("unexpected cache size: %d", len(cv.cache))
	}
}

func TestCachingVerifier_SweepExpired(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Minute).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"
	claims, err := cv.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Sub != token {
		t.Errorf("unexpected claims: %+v", claims)
	}

	clock.Advance(2 * time.Minute)
	cv.evictStale()
	if len(cv.cache) != 0 {
		t.Errorf("expected empty cache after sweep, got len=%d", len(cv.cache))
	}
}

func TestCachingVerifier_Concurrent(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Minute).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}

	const goroutineCount = 100
	var wg sync.WaitGroup
	ctx := context.Background()

	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	defer cv.Stop()

	token := "token"

	_, _ = cv.Verify(ctx, token)
	atomic.StoreInt32(&inner.vCount, 0)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claims, err := cv.Verify(ctx, token)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if claims.Sub != token {
				t.Errorf("unexpected claims: %+v", claims)
			}
		}()
	}

	wg.Wait()

	ic := atomic.LoadInt32(&inner.vCount)
	if ic != 0 {
		t.Errorf("expected 0 inner calls after warm-up, got %d", ic)
	}
}

func TestCachingVerifier_Stop(t *testing.T) {
	goNumStart := runtime.NumGoroutine()

	clock := &fakeClock{t: time.Now()}
	exp := clock.Now().Add(time.Hour).Unix()
	inner := &fakeVerifier{
		exp:    exp,
		err:    nil,
		vCount: 0,
	}
	cv := NewCachingVerifier(inner, 10, time.Minute, time.Minute, clock.Now)
	cv.Stop()
	panicked := didPanic(cv.Stop)
	if panicked {
		t.Error("unexpected panic")
	}

	time.Sleep(10 * time.Millisecond)
	goNumEnd := runtime.NumGoroutine()

	if goNumEnd-goNumStart != 0 {
		t.Errorf("expected no goroutine leaks, but got %d", goNumEnd-goNumStart)
	}
}
