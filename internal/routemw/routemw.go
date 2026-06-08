package routemw

import "api-gateway/internal/router"

type RouteMiddleware func(handler router.Handler) router.Handler

func Chain(h router.Handler, middlewares ...RouteMiddleware) router.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		h = middlewares[i](h)
	}

	return h
}
