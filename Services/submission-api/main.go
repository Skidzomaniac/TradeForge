// Command submission-api receives contestant uploads, stores them in object
// storage, records them in Postgres, and queues build jobs on Kafka.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/submission-api/handlers"
	"github.com/trade-eval/submission-api/kafka"
	"github.com/trade-eval/submission-api/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := initLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("submission-api starting", "port", cfg.Port, "environment", cfg.Environment)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.OrchestratorDBDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer rdb.Close()

	mc, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	submissions := repository.NewPostgresSubmissionRepository(pool)
	tests := repository.NewPostgresTestRepository(pool)
	contestants := repository.NewPostgresContestantRepository(pool, rdb)
	webhooks := repository.NewPostgresWebhookRepository(pool)

	h := handlers.New(handlers.Deps{
		Logger:          logger,
		Submissions:     submissions,
		Tests:           tests,
		Contestants:     contestants,
		Webhooks:        webhooks,
		Minio:           mc,
		Redis:           rdb,
		Producer:        producer,
		Bucket:          cfg.MinIOBucket,
		BuildJobsTopic:  cfg.BuildJobsTopic,
		OrchEventsTopic: cfg.OrchEventsTopic,
		MaxUploadBytes:  cfg.MaxUploadSizeMB * 1024 * 1024,
		StartedAt:       time.Now().Unix(),
		KafkaBrokers:    cfg.KafkaBrokers,
		DBPing:          func() error { return pool.Ping(context.Background()) },
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      buildRouter(logger, h, contestants, rdb),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
		}
	}()
	logger.Info("submission-api listening", "addr", srv.Addr)

	<-sigCh
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
