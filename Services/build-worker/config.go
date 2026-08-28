package main

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds build-worker runtime configuration.
type Config struct {
	KafkaBrokers       string
	BuildJobsTopic     string
	OrchEventsTopic    string
	ConsumerGroupID    string
	OrchestratorDBDSN  string
	RedisURL           string
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	DockerHost         string
	WorkDir            string
	MaxConcurrentBuild int
	ContestantNetwork  string
	SandboxMemoryMB    int64
	SandboxCPUCores    string
	SeccompProfile     string
	AppArmorProfile    string
	StrictSandbox      bool
	LogLevel           string
	Environment        string
}

func loadConfig() Config {
	maxConc, _ := strconv.Atoi(getEnv("MAX_CONCURRENT_BUILDS", "3"))
	if maxConc <= 0 {
		maxConc = 3
	}
	memMB, _ := strconv.ParseInt(getEnv("SANDBOX_MEMORY_MB", "512"), 10, 64)
	if memMB <= 0 {
		memMB = 512
	}
	return Config{
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
		BuildJobsTopic:     getEnv("BUILD_JOBS_TOPIC", "build-jobs"),
		OrchEventsTopic:    getEnv("ORCHESTRATOR_EVENTS_TOPIC", "orchestrator-events"),
		ConsumerGroupID:    getEnv("CONSUMER_GROUP_ID", "build-workers"),
		OrchestratorDBDSN:  getEnv("ORCHESTRATOR_DB_DSN", "postgres://postgres:postgres@localhost:5433/orchestrator?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "localhost:6379"),
		MinIOEndpoint:      getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:     getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:        getEnv("MINIO_BUCKET", "submissions"),
		DockerHost:         getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		WorkDir:            getEnv("WORK_DIR", "/tmp/builds"),
		MaxConcurrentBuild: maxConc,
		ContestantNetwork:  getEnv("CONTESTANT_NETWORK", "contestant-isolated"),
		SandboxMemoryMB:    memMB,
		SandboxCPUCores:    getEnv("SANDBOX_CPU_CORES", "2,3"),
		SeccompProfile:     getEnv("SECCOMP_PROFILE", ""),
		AppArmorProfile:    getEnv("APPARMOR_PROFILE", ""),
		StrictSandbox:      getEnv("SANDBOX_STRICT", "false") == "true",
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
