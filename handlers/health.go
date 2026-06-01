package handlers

import (
	"api-gateway/internal/router"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request, params *router.Params) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
