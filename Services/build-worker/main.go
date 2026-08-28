// Command build-worker consumes build jobs, compiles contestant code into
// hardened Docker sandboxes, and reports readiness.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/build-worker/security"
)

func main() {
	if err := run(); err != nil {
		slog.Error("build-worker failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := initLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("build-worker starting", "max_concurrent", cfg.MaxConcurrentBuild)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

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

	dockerCli, err := client.NewClientWithOpts(
		client.WithHost(cfg.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}
	defer dockerCli.Close()

	producer := NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	cm := NewContainerManager(dockerCli, pool, rdb, logger)
	worker := &Worker{
		cfg:      cfg,
		logger:   logger,
		docker:   dockerCli,
		minio:    mc,
		pool:     pool,
		redis:    rdb,
		producer: producer,
		cm:       cm,
		scanner:  security.NewImageScanner(logger),
		monitor:  security.NewResourceMonitor(dockerCli, logger),
	}

	// Re-register containers from previous build-worker instances so the
	// cleanup job doesn't immediately kill them as orphans.
	cm.RecoverActive(ctx)

	go cm.MonitorHealth(ctx)
	go cm.RunCleanup(ctx)

	consumer := NewConsumer(cfg, logger, worker)
	defer consumer.Close()

	logger.Info("build-worker consuming", "topic", cfg.BuildJobsTopic)
	errCh := make(chan error, 1)
	go func() { errCh <- consumer.Run(ctx) }()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			logger.Error("consumer error", "error", err)
		}
	}

	logger.Info("draining containers")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cm.StopAll(shutdownCtx)
	return nil
}
