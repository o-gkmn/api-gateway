package main

import (
	"api-gateway/handlers"
	"api-gateway/internal/config"
	"api-gateway/internal/middleware"
	"api-gateway/logger"
	"api-gateway/server"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	key, err := loadRSAPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		log.Fatalf("failed to load RSA public key: %v", err)
	}

	logger.Init()
	logger.Info("Starting server...")

	s := server.NewServer(cfg.Addr, cfg.Port, cfg.SSLCertPath, cfg.SSLKeyPath)

	rl := middleware.NewRateLimiter(10, 20)
	v := middleware.NewJWTVerifier(key, cfg.JWTIssuer, cfg.JWTAudience)

	s.Use(middleware.Recovery)
	s.Use(middleware.RequestID)
	s.Use(middleware.Logger)
	s.Use(rl.RateLimit)
	s.Use(middleware.Auth(v))

	logger.Info("Middlewares is ready")

	s.Router.GET("/healthz", handlers.HealthHandler)
	ready := handlers.NewReadyHandler()
	s.Router.GET("/readyz", ready.ServeHTTP)

	logger.Info("Router is ready")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server run error: %v", err)
		}
	}()

	logger.Info("Server is running...")

	<-stop

	fmt.Println("Server is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
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
