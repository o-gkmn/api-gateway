package mw

import (
	"api-gateway/internal/reqctx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogger_PassesThrough(t *testing.T) {
	called := false

	h := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(reqctx.WithRequestID(req.Context(), "abc"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Error("logger was not called")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("got status code %d, want %d", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("got body %q, want %q", rec.Body.String(), "ok")
	}
}
