// Package handlers contains the HTTP handlers for submission-api.
package handlers

import (
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/submission-api/kafka"
	"github.com/trade-eval/submission-api/repository"
)

// Deps bundles everything the handlers need.
type Deps struct {
	Logger          *slog.Logger
	Submissions     repository.SubmissionRepository
	Tests           repository.TestRepository
	Contestants     repository.ContestantRepository
	Webhooks        repository.WebhookRepository
	Minio           *minio.Client
	Redis           *redis.Client
	Producer        *kafka.Producer
	Bucket          string
	BuildJobsTopic  string
	OrchEventsTopic string
	MaxUploadBytes  int64
	StartedAt       int64
	KafkaBrokers    string
	DBPing          func() error
}

// Handlers is the HTTP handler set.
type Handlers struct {
	d Deps
}

// New constructs the handler set.
func New(d Deps) *Handlers { return &Handlers{d: d} }
