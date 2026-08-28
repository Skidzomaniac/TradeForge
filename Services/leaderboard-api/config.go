package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds leaderboard-api runtime configuration.
type Config struct {
	Port            string
	RedisURL        string
	TimescaleDSN    string
	UpdateIntervalMs int
	MaxWSConns      int
	AdminAPIKey     string
	LogLevel        string
	Environment     string
}

func loadConfig() Config {
	upd, _ := strconv.Atoi(getEnv("LEADERBOARD_UPDATE_INTERVAL_MS", "500"))
	if upd <= 0 {
		upd = 500
	}
	maxWS, _ := strconv.Atoi(getEnv("MAX_WEBSOCKET_CONNECTIONS", "10000"))
	if maxWS <= 0 {
		maxWS = 10000
	}
	return Config{
		Port:             getEnv("PORT", "8084"),
		RedisURL:         getEnv("REDIS_URL", "localhost:6379"),
		TimescaleDSN:     getEnv("TIMESCALE_DSN", "postgres://postgres:postgres@localhost:5432/tradeeval?sslmode=disable"),
		UpdateIntervalMs: upd,
		MaxWSConns:       maxWS,
		AdminAPIKey:      getEnv("ADMIN_API_KEY", "admin-dev-key"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		Environment:      getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func initLogger(cfg Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Environment == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
