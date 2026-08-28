package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds bot-fleet runtime configuration.
type Config struct {
	KafkaBrokers     string
	OrchEventsTopic  string
	TelemetryTopic   string
	ConsumerGroupID  string
	MaxBotsPerPod    int
	RequestTimeoutMs int
	MetricsPort      string
	LogLevel         string
	Environment      string
}

func loadConfig() Config {
	maxBots, _ := strconv.Atoi(getEnv("MAX_BOTS_PER_POD", "200"))
	if maxBots <= 0 {
		maxBots = 200
	}
	timeout, _ := strconv.Atoi(getEnv("BOT_REQUEST_TIMEOUT_MS", "5000"))
	if timeout <= 0 {
		timeout = 5000
	}
	return Config{
		KafkaBrokers:     getEnv("KAFKA_BROKERS", "localhost:9092"),
		OrchEventsTopic:  getEnv("ORCHESTRATOR_EVENTS_TOPIC", "orchestrator-events"),
		TelemetryTopic:   getEnv("BOT_TELEMETRY_TOPIC", "bot-telemetry"),
		ConsumerGroupID:  getEnv("CONSUMER_GROUP_ID", "bot-fleet-workers"),
		MaxBotsPerPod:    maxBots,
		RequestTimeoutMs: timeout,
		MetricsPort:      getEnv("METRICS_PORT", "8082"),
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
