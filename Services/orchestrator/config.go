package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds orchestrator runtime configuration.
type Config struct {
	KafkaBrokers       string
	OrchEventsTopic    string
	BotFleetTopic      string
	ConsumerGroupID    string
	OrchestratorDBDSN  string
	RedisURL           string
	InstanceID         string
	HeartbeatSeconds   int
	OrphanDetectSecs   int
	AutoTriggerTests   bool
	DefaultDurationSec int
	DefaultBotCount    int
	HealthPort         string
	AdminAPIKey        string
	LogLevel           string
	Environment        string
}

func loadConfig(instanceID string) Config {
	hb, _ := strconv.Atoi(getEnv("HEARTBEAT_INTERVAL_SECONDS", "10"))
	if hb <= 0 {
		hb = 10
	}
	orphan, _ := strconv.Atoi(getEnv("ORPHAN_DETECTION_INTERVAL_SECONDS", "60"))
	if orphan <= 0 {
		orphan = 60
	}
	dur, _ := strconv.Atoi(getEnv("TEST_DEFAULT_DURATION_SECONDS", "300"))
	bots, _ := strconv.Atoi(getEnv("TEST_DEFAULT_BOT_COUNT", "500"))
	return Config{
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
		OrchEventsTopic:    getEnv("ORCHESTRATOR_EVENTS_TOPIC", "orchestrator-events"),
		BotFleetTopic:      getEnv("BOT_FLEET_TOPIC", "orchestrator-events"),
		ConsumerGroupID:    getEnv("ORCHESTRATOR_CONSUMER_GROUP", "orchestrators"),
		OrchestratorDBDSN:  getEnv("ORCHESTRATOR_DB_DSN", "postgres://postgres:postgres@localhost:5433/orchestrator?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "localhost:6379"),
		InstanceID:         instanceID,
		HeartbeatSeconds:   hb,
		OrphanDetectSecs:   orphan,
		AutoTriggerTests:   getEnv("AUTO_TRIGGER_TESTS", "false") == "true",
		DefaultDurationSec: dur,
		DefaultBotCount:    bots,
		HealthPort:         getEnv("HEALTH_PORT", "8082"),
		AdminAPIKey:        getEnv("ADMIN_API_KEY", "admin-dev-key"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		Environment:        getEnv("ENVIRONMENT", "development"),
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
