package config

import (
	"api-gateway/utils"
	"fmt"
	"time"
)

type Config struct {
	Addr              string
	Port              int
	SSLCertPath       string
	SSLKeyPath        string
	JWTPublicKeyPath  string
	JWTIssuer         string
	JWTAudience       string
	RateLimitRPS      float64
	RateLimitBurst    float64
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	AuthEnabled       bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Addr:              utils.GetEnv("ADDR", ":8080"),
		Port:              utils.GetEnvInt("PORT", 8080),
		SSLCertPath:       utils.GetEnv("SSL_CERT_PATH"),
		SSLKeyPath:        utils.GetEnv("SSL_KEY_PATH"),
		JWTPublicKeyPath:  utils.GetEnv("JWT_PUBLIC_KEY_PATH"),
		JWTIssuer:         utils.GetEnv("JWT_ISSUER"),
		JWTAudience:       utils.GetEnv("JWT_AUDIENCE"),
		RateLimitRPS:      utils.GetEnvFloat("RATE_LIMIT_RPS", 10),
		RateLimitBurst:    utils.GetEnvFloat("RATE_LIMIT_BURST", 20),
		ReadHeaderTimeout: utils.GetEnvDuration("READ_HEADER_TIMEOUT", 5) * time.Second,
		ReadTimeout:       utils.GetEnvDuration("READ_TIMEOUT", 15) * time.Second,
		WriteTimeout:      utils.GetEnvDuration("WRITE_HEADER_TIMEOUT", 15) * time.Second,
		IdleTimeout:       utils.GetEnvDuration("IDLE_TIMEOUT", 60) * time.Second,
		AuthEnabled:       utils.GetEnvBool("AUTH_ENABLED", true),
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

	if cfg.SSLKeyPath == "" {
		return nil, fmt.Errorf("SSL_CERT_PATH is required")
	}

	if cfg.SSLCertPath == "" {
		return nil, fmt.Errorf("SSL_KEY_PATH is required")
	}

	return cfg, nil
}
