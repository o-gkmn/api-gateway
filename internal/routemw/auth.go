package routemw

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"net/http"
	"strings"
)

func Auth(verifier auth.Verifier) func(handler router.Handler) router.Handler {
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
