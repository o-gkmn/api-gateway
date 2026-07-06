package mw

import (
	"api-gateway/internal/reqctx"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRequestID_Concurrent(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := reqctx.GetRequestID(r.Context()); !ok || id == "" {
			t.Error("request id not exist in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	const n = 500
	ids := make([]string, n)

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			ids[i] = rec.Header().Get("X-Request-ID")
		}(i)
	}
	wg.Wait()

	seen := make(map[string]struct{})
	for i, id := range ids {
		if id == "" {
			t.Fatalf("request %d has empty id", i)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("request %d has duplicate id", i)
		}
		seen[id] = struct{}{}
	}
}
