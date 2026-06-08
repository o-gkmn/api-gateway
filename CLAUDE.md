# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose
This repository is a learning project to implement a high-performance HTTP server in Go with custom routing and middleware. 
Do not write code. Just tell me how to work with it and help me understand the architecture and design decisions. 
Explain details about any part of the codebase when I ask. You will be my guide and i will make the implementation decisions and write the code

## Goals
- **Zero allocation on the hot path** — avoid heap allocations per request wherever possible. Prefer stack allocation, sync.Pool, and pre-allocated structures.

## Design Decisions

### Two middleware types — do not unify them
There are intentionally two middleware types:
- `Middleware` = `func(http.Handler) http.Handler` — for global middlewares (Recovery, RequestID, Logger, RateLimit, Auth)
- `RouteMiddleware` = `func(router.Handler) router.Handler` — for per-route middlewares

They must stay separate for two reasons:
1. Global middlewares run **before** routing — `*router.Params` does not exist yet and would always be nil
2. `http.Server.ListenAndServeTLS` requires an `http.Handler`. The global chain wraps the router as `http.Handler` to satisfy this. Changing `Middleware` to wrap `router.Handler` would break stdlib server compatibility and require a hidden adapter anyway.

## Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/router/...
go test ./internal/mw/...

# Run a specific test
go test ./internal/router/ -run TestTreeMatch

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./internal/router/

# Run the server (requires env vars — see Configuration)
JWT_PUBLIC_KEY_PATH=keys/jwt_public.pem \
JWT_ISSUER=your-issuer \
JWT_AUDIENCE=your-audience \
SSL_CERT_PATH=keys/server.crt \
SSL_KEY_PATH=keys/server.key \
go run main.go
```

## Configuration

All config is loaded from environment variables in `internal/config/config.go`. These four are **required** — the server refuses to start without them:

| Variable | Description |
|---|---|
| `JWT_PUBLIC_KEY_PATH` | Path to RSA public key PEM file (used to verify JWTs) |
| `JWT_ISSUER` | Expected JWT `iss` claim |
| `JWT_AUDIENCE` | Expected JWT `aud` claim |
| `SSL_CERT_PATH` | TLS certificate path |
| `SSL_KEY_PATH` | TLS private key path |

Optional: `ADDR` (default `:8080`), `PORT` (default `8080`), `RATE_LIMIT_RPS` (default `10`), `RATE_LIMIT_BURST` (default `20`).

Dev keys are checked in under `keys/` — use them for local testing only.

## Architecture

The server is an HTTPS-only API gateway with a custom radix-tree router and a middleware chain.

**Request flow:**
```
TLS → Recovery → RequestID → Logger → RateLimit (per-IP token bucket) → Auth (JWT RS256) → Router → Handler
```

### Custom Router (`internal/router/`)

A hand-rolled radix tree supporting static, param (`:name`), and wildcard (`*name`) segments. Key constraints:
- Max 8 URL parameters per route (`maxParams` in `router.go`)
- Wildcards must appear at the end of a path
- Trailing slashes are stripped before matching — `/users/` and `/users` resolve to the same route
- `Params` objects are pooled (`sync.Pool`) and **must** be released via `tree.ReleaseParams(params)` after use; the router's `ServeHTTP` does this with `defer`
- Handler signature is `func(w http.ResponseWriter, r *http.Request, params *router.Params)` — not the stdlib `http.HandlerFunc`

### Middleware (`internal/middleware/`)

Middlewares are standard `func(http.Handler) http.Handler`. `middleware.Chain` applies them in the order passed (wraps right-to-left so the first listed runs first). Added to the server with `s.Use(...)`.

- **Auth**: extracts `Bearer` token, verifies RS256 JWT, stores `*reqctx.Claims{Subject, Role}` in context via `reqctx.WithClaims`
- **RateLimiter**: per-IP token bucket; stale entries (> 3 min idle) are evicted every minute by a background goroutine
- **Recovery**: catches panics and returns 500
- **RequestID**: generates/propagates `X-Request-ID`
- **Logger**: structured JSON logging using the `logger` package wrapper around `log/slog`

### Context helpers (`internal/reqctx/`)

`reqctx.GetClaims(ctx)` retrieves the JWT claims stored by the Auth middleware. `reqctx.GetRequestID(ctx)` retrieves the request ID.

### Logger (`logger/`)

Wraps `log/slog` with a context-aware handler. Call `logger.Init()` once at startup. Supports attaching extra `slog.Attr`s to a context with `logger.With(ctx, attrs...)` — they are automatically included in any log line that uses that context.

### Server (`server/`)

`server.NewServer` wires together the router and TLS config. Always runs TLS via `ListenAndServeTLS`. Graceful shutdown is handled in `main.go` with a 5-second timeout on `SIGTERM`/`SIGINT`.

### Response recording (`internal/httpx/`)

`httpx.WrapWriter(w)` wraps an `http.ResponseWriter` to capture status code and bytes written. Implements `http.Hijacker` passthrough when the underlying writer supports it. Used by the Logger middleware.

