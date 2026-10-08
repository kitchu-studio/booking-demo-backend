package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds runtime settings, all read from environment variables.
type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:      getenv("APP_ENV", "development"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		AccessTTL:   getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:  getDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET is required and must be at least 32 characters")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
