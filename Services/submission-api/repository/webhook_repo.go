package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Webhook mirrors the webhooks table.
type Webhook struct {
	ID           string    `db:"id"`
	ContestantID string    `db:"contestant_id"`
	URL          string    `db:"url"`
	Events       []string  `db:"events"`
	Secret       string    `db:"secret"`
	Active       bool      `db:"active"`
	CreatedAt    time.Time `db:"created_at"`
}

// WebhookRepository is the persistence contract for webhooks.
type WebhookRepository interface {
	Create(ctx context.Context, wh *Webhook) error
	ListByContestant(ctx context.Context, contestantID string) ([]*Webhook, error)
	ListForEvent(ctx context.Context, contestantID, event string) ([]*Webhook, error)
}

// PostgresWebhookRepository implements WebhookRepository with pgx.
type PostgresWebhookRepository struct{ pool *pgxpool.Pool }

// NewPostgresWebhookRepository constructs the repository.
func NewPostgresWebhookRepository(pool *pgxpool.Pool) *PostgresWebhookRepository {
	return &PostgresWebhookRepository{pool: pool}
}

func (r *PostgresWebhookRepository) Create(ctx context.Context, wh *Webhook) error {
	const q = `INSERT INTO webhooks (id, contestant_id, url, events, secret, active) VALUES ($1,$2,$3,$4,$5,true)`
	if _, err := r.pool.Exec(ctx, q, wh.ID, wh.ContestantID, wh.URL, wh.Events, wh.Secret); err != nil {
		return fmt.Errorf("webhookRepo.Create: %w", err)
	}
	return nil
}

func (r *PostgresWebhookRepository) ListByContestant(ctx context.Context, contestantID string) ([]*Webhook, error) {
	const q = `SELECT id, contestant_id, url, events, secret, active, created_at FROM webhooks WHERE contestant_id = $1`
	rows, err := r.pool.Query(ctx, q, contestantID)
	if err != nil {
		return nil, fmt.Errorf("webhookRepo.ListByContestant: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Webhook])
}

func (r *PostgresWebhookRepository) ListForEvent(ctx context.Context, contestantID, event string) ([]*Webhook, error) {
	const q = `SELECT id, contestant_id, url, events, secret, active, created_at
	           FROM webhooks WHERE contestant_id = $1 AND active AND $2 = ANY(events)`
	rows, err := r.pool.Query(ctx, q, contestantID, event)
	if err != nil {
		return nil, fmt.Errorf("webhookRepo.ListForEvent: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Webhook])
}
