package main

import (
	"api-gateway/handlers"
	"api-gateway/internal/config"
	"api-gateway/internal/mw"
	"api-gateway/internal/routemw"
	"api-gateway/logger"
	"api-gateway/server"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger.Init()
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	run(cfg)
}

func run(cfg *config.Config) {
	s := server.NewServer(
		cfg.Addr,
		cfg.Port,
		cfg.SSLCertPath,
		cfg.SSLKeyPath,
		cfg.ReadHeaderTimeout,
		cfg.ReadTimeout,
		cfg.WriteTimeout,
		cfg.IdleTimeout,
	)

	if err := registerMiddleware(s, cfg); err != nil {
		logger.Error("failed to register mw", slog.Any("error", err))
		os.Exit(1)
	}

	auth, err := buildAuth(cfg)
	if err != nil {
		logger.Error("failed to build auth", slog.Any("error", err))
		os.Exit(1)
	}

	registerRoutes(s, auth)
	serve(s)
}

func registerMiddleware(s *server.Server, cfg *config.Config) error {
	rl := mw.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	s.Use(mw.Recovery)
	s.Use(mw.RequestID)
	s.Use(mw.Logger)
	s.Use(rl.RateLimit)

	return nil
}

func buildAuth(cfg *config.Config) (routemw.RouteMiddleware, error) {
	if !cfg.AuthEnabled {
		logger.Warn("AUTH IS DISABLED")
		return nil, nil
	}

	key, err := loadRSAPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		logger.Error("failed to load RSA public key", slog.Any("error", err))
		return nil, err
	}
	v := mw.NewJWTVerifier(key, cfg.JWTIssuer, cfg.JWTAudience)
	return mw.Auth(v), nil
}

func registerRoutes(s *server.Server, auth routemw.RouteMiddleware) {
	ready := handlers.NewReadyHandler()

	s.Router.GET("/healthz", handlers.HealthHandler)
	s.Router.GET("/readyz", ready.ServeHTTP)
	s.Router.GET("/whoami", routemw.Chain(handlers.WhoAmIHandler, auth))
}

func serve(s *server.Server) {
	s.Setup()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := s.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	logger.Info("Server is running...")

	select {
	case err := <-errChan:
		logger.Error("Server error", slog.Any("error", err))
		os.Exit(1)
	case <-stop:
		logger.Info("Server is shutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server", slog.Any("error", err))
		return
	}

	logger.Info("Server stopped gracefully")
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read RSA public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to parse RSA public key")
	}
	return rsaPub, nil
}
