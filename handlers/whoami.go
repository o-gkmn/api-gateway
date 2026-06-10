package handlers

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"fmt"
	"net/http"
)

func WhoAmIHandler(w http.ResponseWriter, r *http.Request, params *router.Params) {
	claims, ok := reqctx.GetClaims(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	resp := fmt.Sprintf("sub %s, roles=%v", claims.Subject, claims.Roles)
	w.Write([]byte(resp))
	w.WriteHeader(http.StatusOK)
}
