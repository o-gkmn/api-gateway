package middleware

import (
	"api-gateway/internal/router"
	"api-gateway/logger"
	"api-gateway/server"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type contextKey string

const requestIDKey = contextKey("requestID")

func RequestID() server.Middleware {
	return func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = generateID()
			}

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			ctx = logger.With(ctx, slog.String("request_id", id))

			next(w, r.WithContext(ctx))
		}
	}
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(fmt.Errorf("failed to generate random ID: %s", err))
	}
	return hex.EncodeToString(b)
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
