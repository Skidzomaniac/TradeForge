package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/submission-api/handlers"
	"github.com/trade-eval/submission-api/middleware"
	"github.com/trade-eval/submission-api/repository"
)

func buildRouter(logger *slog.Logger, h *handlers.Handlers, contestants repository.ContestantRepository, rdb *redis.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(middleware.SecurityHeaders)
	r.Use(corsMiddleware)
	r.Use(metricsMiddleware)

	// Public routes.
	r.Get("/v1/health", h.Health)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
	r.Get("/readyz", h.Health)
	r.Handle("/metrics", metricsHandler())
	r.Get("/v1/leaderboard", h.GetLeaderboard)

	// Authenticated routes.
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.Auth(contestants, rdb))
		pr.Post("/v1/submissions", h.CreateSubmission)
		pr.Get("/v1/submissions/{id}", h.GetSubmission)
		pr.Get("/v1/submissions/{id}/logs", h.GetSubmissionLogs)
		pr.Post("/v1/tests", h.CreateTest)
		pr.Get("/v1/tests/{id}", h.GetTest)
		pr.Post("/v1/webhooks", h.CreateWebhook)
		pr.Get("/v1/webhooks", h.ListWebhooks)
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
