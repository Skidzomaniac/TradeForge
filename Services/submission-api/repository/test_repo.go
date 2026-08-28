package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trade-eval/submission-api/apierrors"
)

// TestRepository is the persistence contract for tests.
type TestRepository interface {
	Create(ctx context.Context, t *Test) error
	GetByID(ctx context.Context, id string) (*Test, error)
	GetActiveByContestantID(ctx context.Context, contestantID string) (*Test, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateFinalScore(ctx context.Context, id string, score float64, endedAt time.Time) error
	SetHeartbeat(ctx context.Context, id string) error
}

// PostgresTestRepository implements TestRepository with pgx.
type PostgresTestRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresTestRepository constructs the repository.
func NewPostgresTestRepository(pool *pgxpool.Pool) *PostgresTestRepository {
	return &PostgresTestRepository{pool: pool}
}

func (r *PostgresTestRepository) Create(ctx context.Context, t *Test) error {
	const q = `
		INSERT INTO tests (id, submission_id, contestant_id, status)
		VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, q, t.ID, t.SubmissionID, t.ContestantID, t.Status)
	if err != nil {
		return fmt.Errorf("testRepo.Create: %w", err)
	}
	return nil
}

func (r *PostgresTestRepository) GetByID(ctx context.Context, id string) (*Test, error) {
	const q = `
		SELECT id, submission_id, contestant_id, status, started_at, ended_at,
		       final_score, failure_reason, orchestrator_instance_id, last_heartbeat_at,
		       created_at, updated_at
		FROM tests WHERE id = $1`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("testRepo.GetByID: %w", err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Test])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierrors.ErrNotFound{Message: "test not found"}
		}
		return nil, fmt.Errorf("testRepo.GetByID scan: %w", err)
	}
	return t, nil
}

func (r *PostgresTestRepository) GetActiveByContestantID(ctx context.Context, contestantID string) (*Test, error) {
	const q = `
		SELECT id, submission_id, contestant_id, status, started_at, ended_at,
		       final_score, failure_reason, orchestrator_instance_id, last_heartbeat_at,
		       created_at, updated_at
		FROM tests
		WHERE contestant_id = $1 AND status IN ('pending','running','stopping')
		ORDER BY created_at DESC LIMIT 1`
	rows, err := r.pool.Query(ctx, q, contestantID)
	if err != nil {
		return nil, fmt.Errorf("testRepo.GetActiveByContestantID: %w", err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Test])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierrors.ErrNotFound{Message: "no active test"}
		}
		return nil, fmt.Errorf("testRepo.GetActiveByContestantID scan: %w", err)
	}
	return t, nil
}

func (r *PostgresTestRepository) UpdateStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE tests SET status = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("testRepo.UpdateStatus: %w", err)
	}
	return nil
}

func (r *PostgresTestRepository) UpdateFinalScore(ctx context.Context, id string, score float64, endedAt time.Time) error {
	const q = `UPDATE tests SET final_score = $2, ended_at = $3, status = 'completed' WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, score, endedAt)
	if err != nil {
		return fmt.Errorf("testRepo.UpdateFinalScore: %w", err)
	}
	return nil
}

func (r *PostgresTestRepository) SetHeartbeat(ctx context.Context, id string) error {
	const q = `UPDATE tests SET last_heartbeat_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("testRepo.SetHeartbeat: %w", err)
	}
	return nil
}
