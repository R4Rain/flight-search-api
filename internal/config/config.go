package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

const (
	maxProviderTimeout = 10 * time.Second
	minProviderTimeout = 100 * time.Millisecond
)

type Config struct {
	Port            string
	CacheTTL        time.Duration
	ProviderTimeout time.Duration
	RateLimit       float64
	RateBurst       int
	MaxRetries      int
}

func Load() Config {
	cfg := Config{
		Port:            getEnv("PORT", "8080"),
		CacheTTL:        getDurationEnv("CACHE_TTL", 5*time.Minute),
		ProviderTimeout: getDurationEnv("PROVIDER_TIMEOUT", 2*time.Second),
		RateLimit:       getFloatEnv("RATE_LIMIT", 10),
		RateBurst:       getIntEnv("RATE_BURST", 20),
		MaxRetries:      getIntEnv("MAX_RETRIES", 3),
	}

	// Clamp provider timeout to reasonable bounds
	if cfg.ProviderTimeout < minProviderTimeout {
		slog.Warn("PROVIDER_TIMEOUT too low, clamping to minimum",
			"configured", cfg.ProviderTimeout.String(),
			"minimum", minProviderTimeout.String(),
		)
		cfg.ProviderTimeout = minProviderTimeout
	}
	if cfg.ProviderTimeout > maxProviderTimeout {
		slog.Warn("PROVIDER_TIMEOUT too high, clamping to maximum",
			"configured", cfg.ProviderTimeout.String(),
			"maximum", maxProviderTimeout.String(),
		)
		cfg.ProviderTimeout = maxProviderTimeout
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getFloatEnv(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
