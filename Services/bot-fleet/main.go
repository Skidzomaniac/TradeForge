// Command bot-fleet spawns simulated trading bots that load-test contestant
// order books and stream telemetry to Kafka.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trade-eval/bot-fleet/health"
	"github.com/trade-eval/bot-fleet/metrics"
	"github.com/trade-eval/bot-fleet/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("bot-fleet failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := initLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("bot-fleet starting", "max_bots_per_pod", cfg.MaxBotsPerPod)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	producer := telemetry.NewProducer(cfg.KafkaBrokers, cfg.TelemetryTopic)
	defer producer.Close()

	m := metrics.New()
	hs := health.NewServer()

	consumer := NewConsumer(cfg, producer, logger)
	defer consumer.Close()

	srv := &http.Server{Addr: ":" + cfg.MetricsPort, Handler: hs.Handler(m.Handler())}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server", "error", err)
		}
	}()

	hs.SetReady(true)
	logger.Info("bot-fleet consuming", "topic", cfg.OrchEventsTopic, "metrics_port", cfg.MetricsPort)

	errCh := make(chan error, 1)
	go func() { errCh <- consumer.Run(ctx) }()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			logger.Error("consumer error", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
