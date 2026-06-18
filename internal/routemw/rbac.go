package routemw

import (
	"api-gateway/internal/logger"
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"log/slog"
	"net/http"
)

func RequireAnyRole(roles ...string) func(handler router.Handler) router.Handler {
	if len(roles) == 0 {
		panic("RequireAnyRole middleware requires at least one role")
	}

	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, params *router.Params) {
			claims, ok := reqctx.GetClaims(r.Context())
			if !ok {
				logger.Error("RBAC: claims missing from context — is auth middleware in the chain?",
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method))
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			for _, role := range claims.Roles {
				if _, ok := allowed[role]; ok {
					next(w, r, params)
					return
				}
			}

			http.Error(w, "forbidden", http.StatusForbidden)
		}
	}
}
