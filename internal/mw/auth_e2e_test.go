package mw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type nopWriter struct{ h http.Header }

func (n *nopWriter) Header() http.Header {
	if n.h == nil {
		n.h = make(http.Header)
	}
	return n.h
}
func (n *nopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopWriter) WriteHeader(int)             {}

func TestAuthFlow_EndToEnd(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	v := NewJWTVerifier(&priv.PublicKey, "iss-x", "aud-x")

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
	verifier := NewJWTVerifier(&priv.PublicKey, issuer, audience)

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

func BenchmarkChainAuth_E2E(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatal(err)
	}
	const issuer = "https://auth.local.test"
	const audience = "api-gateway"
	verifier := NewJWTVerifier(&priv.PublicKey, issuer, audience)

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

	rl := NewRateLimiter(1e9, 1e9)
	final := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		w.WriteHeader(http.StatusOK)
	}
	protected := Auth(verifier)(final)

	rt := router.NewRouter()
	rt.GET("/", protected) // ← kendi router kayıt API'ne göre uyarla

	h := Chain(rt, Recovery, RequestID, Logger, rl.RateLimit) // rt http.Handler → global zincir sarar

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Authorization", "Bearer "+signed)
	w := &nopWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}
