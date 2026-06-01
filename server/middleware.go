package server

import (
	"net/http"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

func (s *Server) ApplyMiddleware(h http.Handler) http.Handler {
	if len(s.middlewares) == 0 {
		return h
	}

	return Chain(h, s.middlewares...)
}

func (s *Server) Use(middlewares ...Middleware) {
	s.middlewares = append(s.middlewares, middlewares...)
}
