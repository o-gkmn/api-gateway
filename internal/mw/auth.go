package mw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"context"
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Verifier interface {
	Verify(ctx context.Context, token string) (*reqctx.Claims, error)
}

func Auth(verifier Verifier) func(handler router.Handler) router.Handler {
	return func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, params *router.Params) {
			tok, ok := bearerToken(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", `Bearer"`)
				http.Error(w, "missing or malformed token", http.StatusUnauthorized)
				return
			}

			claims, err := verifier.Verify(r.Context(), tok)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := reqctx.WithClaims(r.Context(), claims)
			next(w, r.WithContext(ctx), params)
		}
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	prefix := "Bearer "
	if len(h) < len(prefix) || h[:len(prefix)] != prefix {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

type JWTVerifier struct {
	key      *rsa.PublicKey
	issuer   string
	audience string
}

func NewJWTVerifier(key *rsa.PublicKey, issuer, audience string) *JWTVerifier {
	return &JWTVerifier{key: key, issuer: issuer, audience: audience}
}

func (v *JWTVerifier) Verify(ctx context.Context, token string) (*reqctx.Claims, error) {
	parsed, err := jwt.Parse(token,
		func(token *jwt.Token) (any, error) { return v.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, err
	}

	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid claims")
	}

	sub, _ := mc["sub"].(string)
	var roles []string
	if raw, ok := mc["roles"].([]any); ok {
		for _, r := range raw {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
	}
	return &reqctx.Claims{Subject: sub, Role: roles}, nil
}
