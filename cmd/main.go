package main

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/logger"
	"api-gateway/internal/mw"
	"api-gateway/internal/proxy"
	"api-gateway/internal/routemw"
	"api-gateway/internal/server"
	"api-gateway/pkg/keys"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var stopFns []func()

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

	authMW, err := buildAuth(cfg)
	if err != nil {
		logger.Error("failed to build auth", slog.Any("error", err))
		os.Exit(1)
	}

	prx := buildProxy(cfg)
	registerRoutes(s, prx, authMW)
	serve(s)
}

func registerMiddleware(s *server.Server, cfg *config.Config) error {
	rl := mw.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	stopFns = append(stopFns, rl.Stop)

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

	var inner auth.Verifier

	if cfg.JWKSEnabled {
		jv := auth.NewJWKSVerifier(
			cfg.JWTIssuer,
			cfg.JWTAudience,
			cfg.JWKSUri,
			cfg.JWKSCooldown,
			cfg.JWKSRefreshInterval,
			nil,
		)
		stopFns = append(stopFns, jv.Stop)
		inner = jv
	} else if cfg.IntrospectionEnabled {
		iv := auth.NewIntrospectionVerifier(
			cfg.IntrospectionEndpoint,
			cfg.IntrospectionClientId,
			cfg.IntrospectionClientSecret,
			cfg.JWTIssuer,
			cfg.JWTAudience,
			nil,
		)
		inner = iv
	} else if cfg.PasetoEnabled {
		key, err := keys.LoadEd25519Public("keys/paseto_public.pem")
		if err != nil {
			return nil, err
		}

		iv := auth.NewPasetoVerifier(cfg.JWTIssuer, cfg.JWTAudience, key, nil)
		inner = iv
	} else {
		key, err := keys.LoadRSAPublicKey(cfg.JWTPublicKeyPath)
		if err != nil {
			logger.Error("failed to load RSA public key", slog.Any("error", err))
			return nil, err
		}
		inner = auth.NewJWTVerifier(key, cfg.JWTIssuer, cfg.JWTAudience)
	}

	cv := auth.NewCachingVerifier(
		inner,
		cfg.AuthCacheSize,
		cfg.AuthCacheTTL,
		cfg.AuthCacheSweepInterval,
		time.Now,
	)
	stopFns = append(stopFns, cv.Stop)

	return routemw.Auth(cv), nil
}

func buildProxy(cfg *config.Config) *proxy.Proxy {
	baseTransport := buildTransport(cfg)
	stopFns = append(stopFns, baseTransport.CloseIdleConnections)

	retryTransport := proxy.NewRetryingTransport(baseTransport, cfg.MaxRetries, nil)

	pool := proxy.NewPool(
		cfg.UpstreamURLs,
		&proxy.RoundRobin{},
		cfg.UpstreamHealthPath,
		cfg.UpstreamThreshold,
		cfg.UpstreamHealthInterval,
	)
	stopFns = append(stopFns, pool.Stop)

	return proxy.NewProxy(pool, retryTransport)
}

func buildTransport(cfg *config.Config) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: cfg.DialKeepAlive,
	}

	return &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     cfg.ForceAttemptHTTP2,
	}
}

func registerRoutes(s *server.Server, prx *proxy.Proxy, authMW routemw.RouteMiddleware) {
	rbac := routemw.RequireAnyRole

	ready := handlers.NewReadyHandler().ServeHTTP
	whoami := routemw.Chain(handlers.WhoAmIHandler, authMW, rbac("admin", "user"))

	s.Router.GET("/healthz", handlers.HealthHandler)
	s.Router.GET("/readyz", ready)
	s.Router.GET("/whoami", whoami)

	s.Router.GET("/users/:id", routemw.Chain(prx.ServeHTTP, authMW, rbac("user", "admin")))
	s.Router.POST("/users/:id", routemw.Chain(prx.ServeHTTP, authMW, rbac("user", "admin")))
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
	}

	for i := len(stopFns) - 1; i >= 0; i-- {
		stopFns[i]()
	}

	logger.Info("Server stopped gracefully")
}
