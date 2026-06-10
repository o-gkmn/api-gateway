package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWKSServer struct {
	mu       sync.Mutex
	server   *httptest.Server
	jwks     JSONWebKeySet
	fetches  int64
	failMode bool
}

func NewJWKSServer() *JWKSServer {
	s := &JWKSServer{}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&s.fetches, 1)

		s.mu.Lock()
		failMode := s.failMode
		keys := s.jwks
		s.mu.Unlock()

		if failMode {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(keys)
	})

	s.server = httptest.NewServer(h)

	return s
}

func (s *JWKSServer) SetJWKS(jwks ...JSONWebKey) {
	s.mu.Lock()
	s.jwks = JSONWebKeySet{Keys: jwks}
	s.mu.Unlock()
}

func (s *JWKSServer) SetFailMode(fail bool) {
	s.mu.Lock()
	s.failMode = fail
	s.mu.Unlock()
}

func (s *JWKSServer) URL() string {
	return s.server.URL
}

func (s *JWKSServer) Fetches() int64 {
	return atomic.LoadInt64(&s.fetches)
}

func (s *JWKSServer) Close() {
	s.server.Close()
}

func signToken(
	key any,
	kid, sub, issuer, audience string,
	signingMethod jwt.SigningMethod,
	exp time.Time,
) (string,
	error) {
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}

	switch signingMethod.(type) {
	case *jwt.SigningMethodHMAC:
		if _, ok := key.([]byte); !ok {
			return "", errors.New("HMAC algoritması için key tipi []byte olmalıdır")
		}
	case *jwt.SigningMethodRSA:
		if _, ok := key.(*rsa.PrivateKey); !ok {
			return "", errors.New("RSA algoritması için key tipi *rsa.PrivateKey olmalıdır")
		}
	}

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func TestJWKSVerifier_Verify(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	token, err := signToken(priv, kid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Subject != sub {
		t.Fatalf("invalid claims subject: %v", claims.Subject)
	}

	if got := s.Fetches(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}
}

func TestJWKSVerifier_OnMissRefresh(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kidA     = "key-A"
		kidB     = "key-B"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kidA))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	token, err := signToken(priv, kidA, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Subject != sub {
		t.Fatalf("invalid claims subject: %v", claims.Subject)
	}
	if got := s.Fetches(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}

	clock.Advance(30 * time.Second)
	priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kidB))

	token, err = signToken(priv, kidB, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err = v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Subject != sub {
		t.Fatalf("invalid claims subject: %v", claims.Subject)
	}

	if got := s.Fetches(); got != 2 {
		t.Fatalf("expected 2 fetch, got %d", got)
	}
}

func TestJWKSVerifier_UnknownKid(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	token, err := signToken(priv, kid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	if claims != nil {
		t.Fatal("expected nil claims")
	}

	if got := s.Fetches(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}
}

func TestJWKSVerifier_CoolDown(t *testing.T) {
	const (
		issuer     = "https://example.com"
		audience   = "api-gateway"
		kid        = "key-A"
		unknownKid = "key-B"
		sub        = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	token, err := signToken(priv, unknownKid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	if claims != nil {
		t.Fatal("expected nil claims")
	}

	if got := s.Fetches(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}

	token, err = signToken(priv, unknownKid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err = v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	if claims != nil {
		t.Fatal("expected nil claims")
	}

	if got := s.Fetches(); got != 1 {
		t.Fatalf("expected 1 fetch, got %d", got)
	}

	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))

	clock.Advance(30 * time.Second)
	token, err = signToken(priv, kid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err = v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Subject != sub {
		t.Fatalf("invalid claims subject: %v", claims.Subject)
	}

	if got := s.Fetches(); got != 2 {
		t.Fatalf("expected 2 fetch, got %d", got)
	}
}

func TestJWKSVerifier_Refresh(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		kidB     = "key-B"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	if v.jwks[kid] == nil {
		t.Fatal("jwks key not found")
	}

	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kidB))
	clock.Advance(30 * time.Second)

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	if v.jwks[kidB] == nil {
		t.Fatal("jwks key not found")
	}

	if len(v.jwks) != 1 {
		t.Fatal("jwks keys not updated")
	}
}

func TestJWKSVerifier_CachePreserved(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kidA     = "key-A"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kidA))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	s.SetFailMode(true)

	clock.Advance(30 * time.Second)

	if err := v.fetchKeys(context.Background()); err == nil {
		t.Fatalf("expected fetch error")
	}

	if v.jwks[kidA] == nil {
		t.Fatal("jwks key not found")
	}
}

func TestJWKSVerifier_MissingKidHeader(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()

	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	token, err := signToken(priv, "", sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error")
	}

	claims, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	if claims != nil {
		t.Fatal("expected nil claims")
	}
}

func TestJWKSVerifier_AlgRestricted(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	secretKey := []byte("super-gizli-api-gateway-secret-anahtari-2026")
	token, err := signToken(secretKey, kid, sub, issuer, audience, jwt.SigningMethodHS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := v.Verify(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}

	if claims != nil {
		t.Fatal("expected nil claims")
	}
}

func TestJWKSVerifier_Concurrent(t *testing.T) {
	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
		kid      = "key-A"
		sub      = "user123"
	)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	s := NewJWKSServer()
	defer s.Close()
	s.SetJWKS(PublicKeyToJWK(&priv.PublicKey, kid))

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	defer v.Stop()

	if err := v.fetchKeys(context.Background()); err != nil {
		t.Fatalf("failed to fetch keys: %v", err)
	}

	token, err := signToken(priv, kid, sub, issuer, audience, jwt.SigningMethodRS256, time.Now().Add(time.Hour))
	if err != nil {
		t.Errorf("failed to sign token: %v", err)
		return
	}

	var wg sync.WaitGroup
	goroutineCount := 100

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			claims, err := v.Verify(context.Background(), token)
			if err != nil {
				t.Errorf("failed to verify token: %v", err)
			}

			if claims == nil {
				t.Errorf("claims is nil")
			}
		}()
	}

	wg.Wait()

	if s.Fetches() != 1 {
		t.Errorf("expected 1 fetches, got %d", s.Fetches())
	}
}

func TestJWKSVerifier_Stop(t *testing.T) {
	goNumStart := runtime.NumGoroutine()

	const (
		issuer   = "https://example.com"
		audience = "api-gateway"
	)

	s := NewJWKSServer()

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := NewJWKSVerifier(issuer, audience, s.URL(), 30*time.Second, time.Hour, clock.Now)
	v.Stop()
	panicked := didPanic(v.Stop)
	s.Close()

	if panicked {
		t.Error("unexpected panic")
	}

	time.Sleep(10 * time.Millisecond)
	goNumEnd := runtime.NumGoroutine()

	if goNumEnd-goNumStart != 0 {
		t.Errorf("expected Go routine to be stopped, got %d", goNumEnd-goNumStart)
	}
}
