package bench

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/mw"
	"api-gateway/internal/routemw"
	"api-gateway/internal/router"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Benchmark_E2E(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	keyPEM, err := os.ReadFile("../keys/jwt_private.pem")
	if err != nil {
		log.Fatalf("read private key (önce ./cmd/genkey çalıştırdın mı?): %v", err)
	}
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	if err != nil {
		log.Fatal(err)
	}

	const issuer = "https://auth.local.test"
	const audience = "api-gateway"
	const jwksUri = "http://localhost:8081/.well-known/jwks.json"
	const cooldown = time.Second * 30
	const refreshInterval = time.Minute * 15

	//inner := auth.NewJWTVerifier(&priv.PublicKey, issuer, audience)
	inner := auth.NewJWKSVerifier(issuer, audience, jwksUri, cooldown, refreshInterval, nil)
	verifier := auth.NewCachingVerifier(inner, 10000, 5*time.Minute, time.Minute, time.Now)
	defer verifier.Stop()

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   "user-123",
		"iss":   issuer,
		"aud":   jwt.ClaimStrings{audience},
		"iat":   jwt.NewNumericDate(now).Unix(),
		"exp":   jwt.NewNumericDate(now.Add(time.Hour)).Unix(),
		"roles": []string{"admin", "user"},
	})
	token.Header["kid"] = "dev"

	signed, err := token.SignedString(priv)
	if err != nil {
		b.Fatal(err)
	}

	rl := mw.NewRateLimiter(1e9, 1e9)
	final := func(w http.ResponseWriter, r *http.Request, p *router.Params) {
		w.WriteHeader(http.StatusOK)
	}
	protected := routemw.Auth(verifier)(routemw.RequireAnyRole("admin")(final))

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
