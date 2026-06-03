package utils

import (
	"os"
	"strconv"
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
