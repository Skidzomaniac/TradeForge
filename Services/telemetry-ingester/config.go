package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds telemetry-ingester runtime configuration.
type Config struct {
	KafkaBrokers    string
	TelemetryTopic  string
	ConsumerGroupID string
	TimescaleDSN    string
	RedisURL        string
	BatchSize       int
	ReorderBufferMs int
	APIPort         string
	InternalToken   string
	LogLevel        string
	Environment     string
}

func loadConfig() Config {
	batch, _ := strconv.Atoi(getEnv("TELEMETRY_BATCH_SIZE", "500"))
	if batch <= 0 {
		batch = 500
	}
	reorder, _ := strconv.Atoi(getEnv("REORDER_BUFFER_MS", "100"))
	if reorder <= 0 {
		reorder = 100
	}
	return Config{
		KafkaBrokers:    getEnv("KAFKA_BROKERS", "localhost:9092"),
		TelemetryTopic:  getEnv("BOT_TELEMETRY_TOPIC", "bot-telemetry"),
		ConsumerGroupID: getEnv("CONSUMER_GROUP_ID", "telemetry-ingesters"),
		TimescaleDSN:    getEnv("TIMESCALE_DSN", "postgres://postgres:postgres@localhost:5432/tradeeval?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "localhost:6379"),
		BatchSize:       batch,
		ReorderBufferMs: reorder,
		APIPort:         getEnv("API_PORT", "8083"),
		InternalToken:   getEnv("INTERNAL_API_TOKEN", "internal-dev-token"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Environment:     getEnv("ENVIRONMENT", "development"),
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
