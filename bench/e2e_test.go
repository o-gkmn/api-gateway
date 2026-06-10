package bench

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/mw"
	"api-gateway/internal/router"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func BenchmarkMW_E2E(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatal(err)
	}
	const issuer = "https://auth.local.test"
	const audience = "api-gateway"

	inner := auth.NewJWTVerifier(&priv.PublicKey, issuer, audience)
	verifier := auth.NewCachingVerifier(inner, 10000, 5*time.Minute, time.Minute, time.Now)
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

	rl := mw.NewRateLimiter(1e9, 1e9)
	final := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		w.WriteHeader(http.StatusOK)
	}
	protected := mw.Auth(verifier)(final)

	rt := router.NewRouter()
	rt.GET("/", protected)

	h := mw.Chain(rt, mw.Recovery, mw.RequestID, mw.Logger, rl.RateLimit)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}
