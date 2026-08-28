package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds all runtime configuration, sourced from the environment.
type Config struct {
	Port              string
	KafkaBrokers      string
	RedisURL          string
	MinIOEndpoint     string
	MinIOAccessKey    string
	MinIOSecretKey    string
	MinIOBucket       string
	MaxUploadSizeMB   int64
	OrchestratorDBDSN string
	BuildJobsTopic    string
	OrchEventsTopic   string
	LogLevel          string
	Environment       string
}

func loadConfig() Config {
	maxUpload, err := strconv.ParseInt(getEnv("MAX_UPLOAD_MB", "50"), 10, 64)
	if err != nil || maxUpload <= 0 {
		maxUpload = 50
	}
	return Config{
		Port:              getEnv("PORT", "8080"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		RedisURL:          getEnv("REDIS_URL", "localhost:6379"),
		MinIOEndpoint:     getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:    getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:    getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:       getEnv("MINIO_BUCKET", "submissions"),
		MaxUploadSizeMB:   maxUpload,
		OrchestratorDBDSN: getEnv("ORCHESTRATOR_DB_DSN", "postgres://postgres:postgres@localhost:5433/orchestrator?sslmode=disable"),
		BuildJobsTopic:    getEnv("BUILD_JOBS_TOPIC", "build-jobs"),
		OrchEventsTopic:   getEnv("ORCHESTRATOR_EVENTS_TOPIC", "orchestrator-events"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		Environment:       getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func initLogger(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)}
	var handler slog.Handler
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
