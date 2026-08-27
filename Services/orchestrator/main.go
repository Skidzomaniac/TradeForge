// Command orchestrator coordinates the test lifecycle state machine, including
// crash recovery and final scoring.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("orchestrator failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	instanceID := uuid.NewString()
	cfg := loadConfig(instanceID)
	logger := initLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("orchestrator starting", "instance", instanceID)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.OrchestratorDBDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer rdb.Close()

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	repo := NewTestRepo(pool)
	mw := NewMetricsWriter(rdb)
	sm := NewStateMachine(cfg, repo, rdb, producer, mw, logger)
	recovery := NewRecovery(cfg, repo, rdb, sm, logger)
	consumer := NewConsumer(cfg, sm, logger)
	defer consumer.Close()

	go recovery.WriteHeartbeats(ctx)
	go recovery.DetectOrphans(ctx)
	go func() {
		if err := consumer.Run(ctx); err != nil {
			logger.Error("consumer stopped", "error", err)
		}
	}()

	srv := &http.Server{Addr: ":" + cfg.HealthPort, Handler: healthHandler(sm, cfg.AdminAPIKey)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server", "error", err)
		}
	}()
	logger.Info("orchestrator ready", "health_port", cfg.HealthPort)

	<-ctx.Done()
	logger.Info("orchestrator shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func healthHandler(sm *TestStateMachine, adminKey string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	// Admin endpoints require a constant-time admin-key match.
	mux.HandleFunc("/admin/active-tests", requireAdmin(adminKey, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sm.ActiveTests())
	}))
	return mux
}

// requireAdmin guards a handler with a constant-time X-Admin-Key check.
func requireAdmin(adminKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Admin-Key")), []byte(adminKey)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
