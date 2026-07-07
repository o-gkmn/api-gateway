package proxy

import (
	"api-gateway/internal/logger"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type RetryingTransport struct {
	inner      http.RoundTripper
	maxRetries int
	backoff    func(attempt int) time.Duration
}

func NewRetryingTransport(transport http.RoundTripper, maxRetries int, backoff func(attempt int) time.Duration) *RetryingTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if backoff == nil {
		backoff = linearBackoff
	}

	return &RetryingTransport{
		inner:      transport,
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

func (t *RetryingTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if !isIdempotent(req.Method) {
		return t.inner.RoundTrip(req)
	}

	if req.Body != nil && req.GetBody == nil {
		return t.inner.RoundTrip(req)
	}

	for attempt := 1; attempt <= t.maxRetries; attempt++ {
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		resp, err = t.inner.RoundTrip(req)

		if !shouldRetry(resp, err, req) {
			return resp, err
		}

		if attempt == t.maxRetries {
			logger.Warn("retry exhausted",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("attempts", attempt))
			return resp, err
		}

		logger.Warn("retrying upstream",
			slog.Int("attempt", attempt),
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path))

		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		select {
		case <-time.After(t.backoff(attempt)):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	return resp, err
}

func linearBackoff(attempt int) time.Duration {
	return time.Duration(attempt) * 100 * time.Millisecond
}

func isIdempotent(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS", "PUT", "DELETE", "TRACE":
		return true
	}
	return false
}

func shouldRetry(resp *http.Response, err error, req *http.Request) bool {
	if req.Context().Err() != nil {
		return false
	}
	if !isIdempotent(req.Method) {
		return false
	}
	if err != nil {
		return true
	}
	if resp != nil && resp.StatusCode >= 500 {
		return true
	}
	return false
}
