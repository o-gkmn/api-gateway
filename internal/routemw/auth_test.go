package routemw

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeVerifier struct {
	claims *reqctx.Claims
	err    error
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	return f.claims, f.err
}

type nopWriter struct{ h http.Header }

func (n *nopWriter) Header() http.Header {
	if n.h == nil {
		n.h = make(http.Header)
	}
	return n.h
}
func (n *nopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopWriter) WriteHeader(int)             {}

func TestAuth_MissingHeader(t *testing.T) {
	called := false
	h := Auth(&fakeVerifier{})(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is missing")
	}

	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header should not be empty")
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	called := false
	h := Auth(&fakeVerifier{})(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "invalid")
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is malformed")
	}

	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header should not be empty")
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	called := false
	verifier := &fakeVerifier{err: errors.New("invalid token")}
	h := Auth(verifier)(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	h(w, r, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if called {
		t.Error("handler should not be called when Authorization header is invalid")
	}
}

func TestAuth_ValidToken_PutsClaimsInContext(t *testing.T) {
	want := &reqctx.Claims{Subject: "test", Roles: []string{"admin"}}
	var got *reqctx.Claims
	called := false

	verifier := &fakeVerifier{claims: want}
	h := Auth(verifier)(func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		called = true
		got, _ = reqctx.GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer test")
	h(w, r, nil)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want %d", w.Code, http.StatusOK)
	}

	if !called {
		t.Error("handler should not be called when Authorization header is valid")
	}

	if got.Subject != want.Subject {
		t.Errorf("got %s, want %s", got.Subject, want.Subject)
	}
}

func TestAuth_E2E(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	v := auth.NewJWTVerifier(&priv.PublicKey, "iss-x", "aud-x")

	whoami := func(w http.ResponseWriter, r *http.Request, params *router.Params) {
		c, _ := reqctx.GetClaims(r.Context())
		fmt.Fprintf(w, "sub=%s", c.Subject)
	}

	protected := Auth(v)(whoami)

	sign := func(claims jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(priv)
		if err != nil {
			t.Fatalf("failed to sign JWT: %v", err)
		}
		return s
	}

	base := jwt.MapClaims{
		"sub": "user_1", "iss": "iss-x", "aud": "aud-x",
		"exp": time.Now().Add(time.Hour).Unix(),
	}

	do := func(authHeader string) (int, string) {
		req, _ := http.NewRequest("GET", "/whoami", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		protected(w, req, nil)
		return w.Code, w.Body.String()
	}

	if code, body := do("Bearer " + sign(base)); code != http.StatusOK || body != "sub=user_1" {
		t.Errorf("expected 200 OK with correct subject, got %d with body: %s", code, body)
	}

	if code, _ := do(""); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}

	if code, _ := do("Bearer invalid"); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}

	expired := jwt.MapClaims{
		"sub": "user_1", "iss": "iss-x", "aud": "aud-x",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	if code, _ := do("Bearer " + sign(expired)); code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", code)
	}
}

func BenchmarkAuth_E2E(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatal(err)
	}
	const issuer = "https://auth.local.test"
	const audience = "api-gateway"

	// To test performance WITHOUT caching, comment out NewCachingVerifier
	// and directly use NewJWTVerifier below:
	// verifier := auth.NewJWTVerifier(&priv.PublicKey, issuer, audience)
	inner := auth.NewJWTVerifier(&priv.PublicKey, issuer, audience)
	verifier := auth.NewCachingVerifier(inner, 1000, 5*time.Minute, time.Minute, time.Now)
	defer verifier.Stop()

	now := time.Now()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString(priv)
	if err != nil {
		b.Fatal(err)
	}

	final := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		w.WriteHeader(http.StatusOK)
	}
	protected := Auth(verifier)(final) // ← auth GERÇEKTEN burada sarılıyor

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := &nopWriter{}
	var params *router.Params // route param yok; final/Auth dokunmuyor, nil yeterli

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protected(w, req, params)
	}
}
