package routemw

import (
	"api-gateway/internal/reqctx"
	"api-gateway/internal/router"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type Validator interface {
	Validate() ValidationErrors
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []ValidationError

func Validate[T Validator]() RouteMiddleware {
	return func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, params *router.Params) {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
					http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			var v T
			err = json.Unmarshal(body, &v)
			if err != nil {
				http.Error(w, "Invalid JSON format", http.StatusBadRequest)
				return
			}

			e := v.Validate()
			if len(e) > 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(e)
				return
			}

			ctx := reqctx.WithPayload(r.Context(), v)
			next(w, r.WithContext(ctx), params)
		}
	}
}
