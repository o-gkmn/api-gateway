package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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

func TestIntrospectionVerifier_Verify(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)
	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims.Subject != "sub123" {
		t.Errorf("expected subject to be sub123, got %v", claims.Subject)
	}
}

func TestIntrospectionVerifier_Verify_Fail(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)
	resp.Active = false

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_AuthServerError(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	oauthServer.SetFailMode(true)

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_AuthHeaderCheck(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	oauthServer.SetAuthCheck(true)

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims == nil {
		t.Fatalf("expected claims, got nil")
	}
}

func TestIntrospectionVerifier_IssAudMismatch(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := NewIntrospectionVerifier(oauthServer.URL(), resp.ClientID, "sec", "iss", "aud", clock.Now)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_ExpiredToken(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)
	resp.Exp = clock.Now().Add(-1 * time.Hour).Unix()

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	claims, err := verifier.Verify(t.Context(), "valid-token")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if claims != nil {
		t.Fatalf("expected nil claims, got %v", claims)
	}
}

func TestIntrospectionVerifier_NoCachingFetchesEveryTime(t *testing.T) {
	clock := &fakeClock{t: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	resp := defaultIntrospectionResponse(clock)

	oauthServer := NewOAuth2Server(resp)
	defer oauthServer.Close()

	verifier := oauthServer.NewTestVerifier(resp, clock)

	for i := 0; i < 3; i++ {
		if _, err := verifier.Verify(t.Context(), "valid-token"); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}

	if got := oauthServer.Fetches(); got != 3 {
		t.Errorf("expected 3 fetches, got %d", got)
	}
}
