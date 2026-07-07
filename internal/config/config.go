package config

import (
	"api-gateway/internal/utils"
	"fmt"
	"time"
)

type Config struct {
	Addr                      string
	Port                      int
	SSLCertPath               string
	SSLKeyPath                string
	AuthEnabled               bool
	AuthCacheSize             int
	AuthCacheTTL              time.Duration
	AuthCacheSweepInterval    time.Duration
	JWTPublicKeyPath          string
	JWTIssuer                 string
	JWTAudience               string
	JWKSEnabled               bool
	JWKSUri                   string
	JWKSCooldown              time.Duration
	JWKSRefreshInterval       time.Duration
	IntrospectionEnabled      bool
	IntrospectionEndpoint     string
	IntrospectionClientId     string
	IntrospectionClientSecret string
	PasetoEnabled             bool
	PasetoPublicKeyPath       string
	RateLimitRPS              float64
	RateLimitBurst            float64
	ReadHeaderTimeout         time.Duration
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	IdleTimeout               time.Duration
	DialTimeout               time.Duration
	DialKeepAlive             time.Duration
	MaxIdleConns              int
	MaxIdleConnsPerHost       int
	MaxConnsPerHost           int
	IdleConnTimeout           time.Duration
	ResponseHeaderTimeout     time.Duration
	ForceAttemptHTTP2         bool
	UpstreamURLs              []string
	UpstreamHealthPath        string
	UpstreamThreshold         int32
	UpstreamHealthInterval    time.Duration
	MaxRetries                int
}

func Load() (*Config, error) {
	cfg := &Config{
		Addr:                      utils.GetEnv("ADDR", ":8080"),
		Port:                      utils.GetEnvInt("PORT", 8080),
		SSLCertPath:               utils.GetEnv("SSL_CERT_PATH"),
		SSLKeyPath:                utils.GetEnv("SSL_KEY_PATH"),
		AuthEnabled:               utils.GetEnvBool("AUTH_ENABLED", true),
		AuthCacheSize:             utils.GetEnvInt("AUTH_CACHE_SIZE", 10000),
		AuthCacheTTL:              utils.GetEnvDuration("AUTH_CACHE_TTL", 5),
		AuthCacheSweepInterval:    utils.GetEnvDuration("AUTH_CACHE_SWEEP_INTERVAL", 1),
		JWTPublicKeyPath:          utils.GetEnv("JWT_PUBLIC_KEY_PATH"),
		JWTIssuer:                 utils.GetEnv("JWT_ISSUER"),
		JWTAudience:               utils.GetEnv("JWT_AUDIENCE"),
		JWKSEnabled:               utils.GetEnvBool("JWKS_ENABLED", false),
		JWKSUri:                   utils.GetEnv("JWKS_URI", ""),
		JWKSCooldown:              utils.GetEnvDuration("JWKS_COOLDOWN", 30) * time.Second,
		JWKSRefreshInterval:       utils.GetEnvDuration("JWKS_REFRESH_INTERVAL", 15) * time.Minute,
		IntrospectionEnabled:      utils.GetEnvBool("INTROSPECTION_ENABLED", false),
		IntrospectionEndpoint:     utils.GetEnv("INTROSPECTION_ENDPOINT", ""),
		IntrospectionClientId:     utils.GetEnv("INTROSPECTION_CLIENT_ID", ""),
		IntrospectionClientSecret: utils.GetEnv("INTROSPECTION_CLIENT_SECRET", ""),
		PasetoEnabled:             utils.GetEnvBool("PASETO_ENABLED", false),
		PasetoPublicKeyPath:       utils.GetEnv("PASETO_PUBLIC_KEY_PATH", ""),
		RateLimitRPS:              utils.GetEnvFloat("RATE_LIMIT_RPS", 10),
		RateLimitBurst:            utils.GetEnvFloat("RATE_LIMIT_BURST", 20),
		ReadHeaderTimeout:         utils.GetEnvDuration("READ_HEADER_TIMEOUT", 5) * time.Second,
		ReadTimeout:               utils.GetEnvDuration("READ_TIMEOUT", 15) * time.Second,
		WriteTimeout:              utils.GetEnvDuration("WRITE_HEADER_TIMEOUT", 15) * time.Second,
		IdleTimeout:               utils.GetEnvDuration("IDLE_TIMEOUT", 60) * time.Second,
		DialTimeout:               utils.GetEnvDuration("DIAL_TIMEOUT", 5) * time.Second,
		DialKeepAlive:             utils.GetEnvDuration("DIAL_KEEP_ALIVE", 30) * time.Second,
		MaxIdleConns:              utils.GetEnvInt("MAX_IDLE_CONNS", 100),
		MaxIdleConnsPerHost:       utils.GetEnvInt("MAX_IDLE_CONNS_PER_HOST", 20),
		MaxConnsPerHost:           utils.GetEnvInt("MAX_CONNS_PER_HOST", 50),
		IdleConnTimeout:           utils.GetEnvDuration("IDLE_CONN_TIMEOUT", 90) * time.Second,
		ResponseHeaderTimeout:     utils.GetEnvDuration("RESPONSE_HEADER_TIMEOUT", 10) * time.Second,
		ForceAttemptHTTP2:         utils.GetEnvBool("FORCE_ATTEMPT_HTTP2", true),
		UpstreamURLs:              utils.GetEnvSlice("UPSTREAM_URLS", []string{}),
		UpstreamHealthPath:        utils.GetEnv("UPSTREAM_HEALTH_PATH", "/health"),
		UpstreamThreshold:         utils.GetEnvInt32("UPSTREAM_THRESHOLD", 3),
		UpstreamHealthInterval:    utils.GetEnvDuration("UPSTREAM_HEALTH_INTERVAL", 10) * time.Second,
		MaxRetries:                utils.GetEnvInt("MAX_RETRIES", 3),
	}

	if cfg.AuthEnabled {
		if cfg.JWTPublicKeyPath == "" {
			return nil, fmt.Errorf("JWT_PUBLIC_KEY_PATH is required")
		}

		if cfg.JWTAudience == "" {
			return nil, fmt.Errorf("JWT_AUDIENCE is required")
		}

		if cfg.JWTIssuer == "" {
			return nil, fmt.Errorf("JWT_ISSUER is required")
		}
	}

	if cfg.IntrospectionEnabled {
		if cfg.IntrospectionEndpoint == "" {
			return nil, fmt.Errorf("INTROSPECTION_ENDPOINT is required")
		}

		if cfg.IntrospectionClientId == "" {
			return nil, fmt.Errorf("INTROSPECTION_CLIENT_ID is required")
		}

		if cfg.IntrospectionClientSecret == "" {
			return nil, fmt.Errorf("INTROSPECTION_CLIENT_SECRET is required")
		}
	}

	if cfg.JWKSEnabled {
		if cfg.JWKSUri == "" {
			return nil, fmt.Errorf("JWKS_URI is required")
		}
	}

	if cfg.PasetoEnabled {
		if cfg.PasetoPublicKeyPath == "" {
			return nil, fmt.Errorf("PASETO_PUBLIC_KEY_PATH is required")
		}
	}

	if cfg.SSLKeyPath == "" {
		return nil, fmt.Errorf("SSL_CERT_PATH is required")
	}

	if cfg.SSLCertPath == "" {
		return nil, fmt.Errorf("SSL_KEY_PATH is required")
	}

	return cfg, nil
}
