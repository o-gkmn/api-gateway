package handlers

import (
	"api-gateway/internal/router"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Checker interface {
	Check(ctx context.Context) error
}

type CheckerFunc func(ctx context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

type ReadyHandler struct {
	mu      sync.RWMutex
	checks  map[string]Checker
	timeout time.Duration
}

func NewReadyHandler() *ReadyHandler {
	return &ReadyHandler{
		checks:  make(map[string]Checker),
		timeout: 2 * time.Second,
	}
}

func (h *ReadyHandler) Register(name string, c Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = c
}

func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, params *router.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	h.mu.RLock()
	checks := make(map[string]Checker, len(h.checks))
	for name, c := range h.checks {
		checks[name] = c
	}
	h.mu.RUnlock()

	results := make(map[string]string, len(checks))
	allOK := true

	for name, check := range checks {
		if err := check.Check(ctx); err != nil {
			results[name] = err.Error()
			allOK = false
			continue
		}
		results[name] = "ok"
	}

	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(results)
}
