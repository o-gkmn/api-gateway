package server

import (
	"api-gateway/internal/logger"
	"api-gateway/internal/mw"
	"api-gateway/internal/router"
	"context"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	addr              string
	port              int
	certPath          string
	keyPath           string
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	server            *http.Server
	Router            *router.Router
	middlewares       []mw.Middleware
	errorHandler      ErrorHandler
}

type Option func(*Server)

func NewServer(
	addr string,
	port int,
	certPath, keyPath string,
	readHeaderTimeout, readTimeout, writeTimeout, idleTimeout time.Duration) *Server {
	s := &Server{
		addr:              addr,
		port:              port,
		certPath:          certPath,
		keyPath:           keyPath,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		idleTimeout:       idleTimeout,
		middlewares:       []mw.Middleware{},
	}

	s.Router = router.NewRouter()

	return s
}

func (s *Server) Setup() {
	addr := fmt.Sprintf("%s:%d", s.addr, s.port)

	h := mw.Chain(s.Router, s.middlewares...)
	s.server = &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: s.readHeaderTimeout,
		ReadTimeout:       s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       s.idleTimeout,
	}
}

func (s *Server) Serve() error {
	logger.Info(fmt.Sprintf("Listening on %s", s.server.Addr))
	if s.certPath != "" && s.keyPath != "" {
		return s.server.ListenAndServeTLS(s.certPath, s.keyPath)
	}
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) GetRouter() *router.Router {
	return s.Router
}

func (s *Server) ApplyMiddleware(h http.Handler) http.Handler {
	return mw.Chain(h, s.middlewares...)
}

func (s *Server) Use(middlewares ...mw.Middleware) {
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
