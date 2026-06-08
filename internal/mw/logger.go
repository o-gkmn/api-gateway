package mw

import (
	"api-gateway/internal/httpx"
	"api-gateway/internal/reqctx"
	"api-gateway/logger"
	"log/slog"
	"net/http"
	"time"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := reqctx.RequestID(r.Context())
		start := time.Now()

		wrapper := httpx.WrapWriter(w)
		next.ServeHTTP(wrapper, r)

		logger.Info("request completed",
			slog.String("request_id", id),
			slog.String("method", r.Method),
			slog.String("uri", r.RequestURI),
			slog.Int("status", wrapper.Status()),
			slog.Int("bytes", wrapper.Bytes()),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
