package server

import (
	"api-gateway/internal/middleware"
	"api-gateway/internal/router"
	"api-gateway/logger"
	"context"
	"fmt"
	"net/http"
)

type Server struct {
	addr         string
	port         int
	certPath     string
	keyPath      string
	server       *http.Server
	Router       *router.Router
	middlewares  []middleware.Middleware
	errorHandler ErrorHandler
}

type Option func(*Server)

func NewServer(addr string, port int, certPath, keyPath string) *Server {
	s := &Server{
		addr:        addr,
		port:        port,
		certPath:    certPath,
		keyPath:     keyPath,
		middlewares: []middleware.Middleware{},
	}

	s.Router = router.NewRouter()

	return s
}

func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.addr, s.port)

	h := middleware.Chain(s.Router, s.middlewares...)

	s.server = &http.Server{
		Addr:    addr,
		Handler: h,
	}

	logger.Info(fmt.Sprintf("Listening on %s", addr))
	return s.server.ListenAndServeTLS(s.certPath, s.keyPath)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) GetRouter() *router.Router {
	return s.Router
}

func (s *Server) ApplyMiddleware(h http.Handler) http.Handler {
	return middleware.Chain(h, s.middlewares...)
}

func (s *Server) Use(middlewares ...middleware.Middleware) {
	s.middlewares = append(s.middlewares, middlewares...)
}

type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

func WithErrorHandler(handler ErrorHandler) Option {
	return func(s *Server) {
		s.errorHandler = handler
	}
}

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
