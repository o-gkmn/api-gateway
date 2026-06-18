package utils

import (
	"os"
	"strconv"
	"time"
)

func GetEnv(key string, fallback ...string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	if len(fallback) > 0 {
		return fallback[0]
	}

	return ""
}

func GetEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		result, err := strconv.Atoi(value)
		if err != nil {
			return fallback
		}
		return result
	}
	return fallback
}

func GetEnvFloat(key string, fallback float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		result, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fallback
		}
		return result
	}
	return fallback
}

func GetEnvDuration(key string, fallback int) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		result, err := strconv.Atoi(value)
		if err != nil {
			return time.Duration(fallback)
		}
		return time.Duration(result)
	}
	return time.Duration(fallback)
}

func GetEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		result, err := strconv.ParseBool(value)
		if err != nil {
			return fallback
		}
		return result
	}
	return fallback
}
