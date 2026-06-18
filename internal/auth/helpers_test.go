package auth

import (
	"api-gateway/internal/reqctx"
	"api-gateway/pkg/keys"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func didPanic(f func()) bool {
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		f()
	}()
	return panicked
}

// Fake Clock
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

// Fake Verifier
type fakeVerifier struct {
	err    error
	exp    int64
	vCount int32
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	atomic.AddInt32(&f.vCount, 1)
	if f.err != nil {
		return nil, f.err
	}

	return &reqctx.Claims{
		Sub:   token,
		Roles: []string{"admin"},
		Exp:   f.exp,
	}, nil
}

// jwks_test.go
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

// introspection_test.go
type OAuth2Server struct {
	mu             sync.Mutex
	server         *httptest.Server
	resp           IntrospectionResponse
	fetches        int64
	failMode       bool
	authCheck      bool
	expectedUser   string
	expectedSecret string
}

func NewOAuth2Server(resp IntrospectionResponse) *OAuth2Server {
	s := &OAuth2Server{}
	s.resp = resp
	s.expectedUser = "user"
	s.expectedSecret = "secret"

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		fail := s.failMode
		resp := s.resp
		authCheck := s.authCheck
		s.mu.Unlock()

		if authCheck {
			user, pass, ok := r.BasicAuth()
			if !ok || user != s.expectedUser || pass != s.expectedSecret {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		atomic.AddInt64(&s.fetches, 1)

		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))

	return s
}

func (s *OAuth2Server) SetFailMode(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failMode = fail
}

func (s *OAuth2Server) SetAuthCheck(authCheck bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authCheck = authCheck
}

func (s *OAuth2Server) URL() string {
	return s.server.URL
}

func (s *OAuth2Server) Fetches() int64 {
	return atomic.LoadInt64(&s.fetches)
}

func (s *OAuth2Server) Close() {
	s.server.Close()
}

func defaultIntrospectionResponse(clock *fakeClock) IntrospectionResponse {
	const (
		clientId = "clientId"
		username = "username"
		scope    = "scope"
		sub      = "sub123"
		aud      = "aud123"
		iss      = "iss123"
		jti      = "jti123"
	)

	resp := IntrospectionResponse{
		Active:   true,
		ClientID: clientId,
		Username: username,
		Scope:    scope,
		Exp:      clock.Now().Add(1 * time.Hour).Unix(),
		IAT:      clock.Now().Add(-1 * time.Hour).Unix(),
		NBF:      clock.Now().Add(-1 * time.Hour).Unix(),
		Sub:      sub,
		Aud:      aud,
		Iss:      iss,
		JTI:      jti,
	}

	return resp
}

func (s *OAuth2Server) NewTestVerifier(resp IntrospectionResponse,
	clock *fakeClock) *IntrospectionVerifier {
	return NewIntrospectionVerifier(s.server.URL, s.expectedUser, s.expectedSecret, resp.Iss, resp.Aud, clock.Now)
}

func setupPasetoVerifier(t *testing.T, issuer, audience string) (*PasetoVerifier, string, *fakeClock) {
	token := "v4.public.eyJhdWQiOiJhcGktZ2F0ZXdheSIsImV4cCI6MTc4MTcwNzA2MywiaXNzIjoiaHR0cHM6Ly9hdXRoLmxvY2FsLnRlc3QiLCJyb2xlcyI6WyJhZG1pbiJdLCJzdWIiOiJ1c2VyXzEifXCpuKOv_lS7qhixOLVH-iciSXU4AnyoPXgT7uWD_xjOO2uPqe6wBrjDVKHTc6dSfZAumEvCP2ndIhJjSl-nggw"

	pubKey, err := keys.LoadEd25519Public("testdata/paseto_public.pem")
	if err != nil {
		t.Fatal(err)
	}

	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}

	v := &PasetoVerifier{
		key:      pubKey,
		issuer:   issuer,
		audience: audience,
		now:      clock.Now,
	}

	return v, token, clock
}
