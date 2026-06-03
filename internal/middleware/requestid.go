package middleware

import (
	"api-gateway/internal/reqctx"
	"api-gateway/logger"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}

		w.Header().Set("X-Request-ID", id)
		ctx := reqctx.WithRequestID(r.Context(), id)
		ctx = logger.With(ctx, slog.String("request_id", id))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(fmt.Errorf("failed to generate random ID: %s", err))
	}
	return hex.EncodeToString(b)
}
