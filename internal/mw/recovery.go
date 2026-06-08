package mw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/logger"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}

			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			id, _ := reqctx.RequestID(r.Context())
			logger.Error("panic recovered",
				slog.String("request_id", id),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			w.WriteHeader(http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
