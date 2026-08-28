// Command leaderboard-api computes the live leaderboard, serves it over REST,
// and streams updates to browsers over WebSocket.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/leaderboard-api/admin"
	"github.com/trade-eval/leaderboard-api/api"
	"github.com/trade-eval/leaderboard-api/commentary"
	"github.com/trade-eval/leaderboard-api/hub"
	"github.com/trade-eval/leaderboard-api/middleware"
	"github.com/trade-eval/leaderboard-api/pubsub"
	"github.com/trade-eval/leaderboard-api/scorer"
)

func main() {
	if err := run(); err != nil {
		slog.Error("leaderboard-api failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := initLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("leaderboard-api starting", "port", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer rdb.Close()

	h := hub.NewHub(cfg.MaxWSConns, logger)
	go h.Run()

	sc := scorer.New(rdb, h, time.Duration(cfg.UpdateIntervalMs)*time.Millisecond, logger)
	gen := commentary.New()
	sc.SetCommentary(func(entries []scorer.Entry) [][]byte {
		evs := gen.Diff(entries)
		out := make([][]byte, 0, len(evs))
		for _, e := range evs {
			if b, err := json.Marshal(e); err == nil {
				out = append(out, b)
			}
		}
		return out
	})
	sc.SetPredictor(scorer.NewPredictor())
	go sc.Run(ctx)

	sub := pubsub.New(rdb, h)
	go sub.Run(ctx)
	go sampleWSConnections(ctx, h)

	handlers := api.New(rdb, sc)
	adminH := admin.New(rdb, cfg.AdminAPIKey)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", handlers.Health)
	mux.HandleFunc("/v1/leaderboard", handlers.GetLeaderboard)
	mux.HandleFunc("/v1/health", handlers.Health)
	mux.HandleFunc("/v1/contestants/", contestantRouter(handlers))
	mux.HandleFunc("/admin/v1/leaderboard/freeze", adminH.Auth(adminH.Freeze))
	mux.HandleFunc("/admin/v1/leaderboard/unfreeze", adminH.Auth(adminH.Unfreeze))
	mux.HandleFunc("/admin/v1/contestants/disqualify", adminH.Auth(adminH.Disqualify))
	mux.HandleFunc("/admin/v1/system/status", adminH.Auth(adminH.SystemStatus))

	handler := middleware.CORS(middleware.RateLimit(rdb, 60)(mux))

	srv := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     handler,
		ReadTimeout: 15 * time.Second,
		IdleTimeout: 3600 * time.Second, // long-lived WS
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", "error", err)
		}
	}()
	logger.Info("leaderboard-api ready", "addr", srv.Addr)

	<-ctx.Done()
	logger.Info("leaderboard-api shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// contestantRouter dispatches /v1/contestants/{id}/{anomalies|insights}.
func contestantRouter(h *api.Handlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/insights"):
			h.GetInsights(w, r)
		case strings.HasSuffix(r.URL.Path, "/prediction"):
			h.GetPrediction(w, r)
		default:
			h.GetAnomalies(w, r)
		}
	}
}
