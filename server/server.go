package server

import (
	"api-gateway/internal/router"
	"api-gateway/logger"
	"api-gateway/utils"
	"context"
	"fmt"
	"net/http"
)

type Server struct {
	port         int
	Router       *router.Router
	server       *http.Server
	middlewares  []Middleware
	errorHandler ErrorHandler
}

type Option func(*Server)

func NewServer(port int) *Server {
	s := &Server{
		port:        port,
		middlewares: []Middleware{},
	}

	s.Router = router.NewRouter()

	return s
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%d", s.port)

	h := Chain(s.Router.ServeHTTP, s.middlewares...)

	s.server = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(h),
	}

	certPath := utils.GetEnv("SSL_CERT_PATH", "")
	keyPath := utils.GetEnv("SSL_KEY_PATH", "")

	logger.Info(fmt.Sprintf("Listening on port %d", s.port))
	return s.server.ListenAndServeTLS(certPath, keyPath)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) GetRouter() *router.Router {
	return s.Router
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
