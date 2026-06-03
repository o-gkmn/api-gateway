package middleware

import (
	"api-gateway/internal/reqctx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovery(t *testing.T) {
	called := false
	h := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx := reqctx.WithRequestID(r.Context(), "test-request-id")
	h.ServeHTTP(w, r.WithContext(ctx))

	if !called {
		t.Errorf("should call recovery")
	}

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
