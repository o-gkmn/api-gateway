package server

import "api-gateway/internal/router"

type Middleware func(router.Handler) router.Handler

func Chain(h router.Handler, middlewares ...Middleware) router.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

func (s *Server) ApplyMiddleware(h router.Handler) router.Handler {
	if len(s.middlewares) == 0 {
		return h
	}

	return Chain(h, s.middlewares...)
}

func (s *Server) Use(middlewares ...Middleware) {
	s.middlewares = append(s.middlewares, middlewares...)
}
