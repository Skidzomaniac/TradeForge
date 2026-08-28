package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trade-eval/submission-api/apierrors"
)

// SubmissionRepository is the persistence contract for submissions.
type SubmissionRepository interface {
	Create(ctx context.Context, s *Submission) error
	GetByID(ctx context.Context, id string) (*Submission, error)
	GetByContestantID(ctx context.Context, contestantID string) ([]*Submission, error)
	UpdateStatus(ctx context.Context, id, status, errorLog string) error
	UpdateContainerInfo(ctx context.Context, id, containerIP string, containerPort int, containerID string) error
}

// PostgresSubmissionRepository implements SubmissionRepository with pgx.
type PostgresSubmissionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSubmissionRepository constructs the repository.
func NewPostgresSubmissionRepository(pool *pgxpool.Pool) *PostgresSubmissionRepository {
	return &PostgresSubmissionRepository{pool: pool}
}

func (r *PostgresSubmissionRepository) Create(ctx context.Context, s *Submission) error {
	const q = `
		INSERT INTO submissions (id, contestant_id, contestant_name, language, s3_key, status)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, q, s.ID, s.ContestantID, s.ContestantName, s.Language, s.S3Key, s.Status)
	if err != nil {
		return fmt.Errorf("submissionRepo.Create: %w", err)
	}
	return nil
}

func (r *PostgresSubmissionRepository) GetByID(ctx context.Context, id string) (*Submission, error) {
	const q = `
		SELECT id, contestant_id, contestant_name, language, s3_key, status,
		       container_ip, container_port, container_id, error_log, created_at, updated_at
		FROM submissions WHERE id = $1`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("submissionRepo.GetByID: %w", err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Submission])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierrors.ErrNotFound{Message: "submission not found"}
		}
		return nil, fmt.Errorf("submissionRepo.GetByID scan: %w", err)
	}
	return s, nil
}

func (r *PostgresSubmissionRepository) GetByContestantID(ctx context.Context, contestantID string) ([]*Submission, error) {
	const q = `
		SELECT id, contestant_id, contestant_name, language, s3_key, status,
		       container_ip, container_port, container_id, error_log, created_at, updated_at
		FROM submissions WHERE contestant_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, contestantID)
	if err != nil {
		return nil, fmt.Errorf("submissionRepo.GetByContestantID: %w", err)
	}
	subs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Submission])
	if err != nil {
		return nil, fmt.Errorf("submissionRepo.GetByContestantID scan: %w", err)
	}
	return subs, nil
}

func (r *PostgresSubmissionRepository) UpdateStatus(ctx context.Context, id, status, errorLog string) error {
	const q = `UPDATE submissions SET status = $2, error_log = NULLIF($3, '') WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, status, errorLog)
	if err != nil {
		return fmt.Errorf("submissionRepo.UpdateStatus: %w", err)
	}
	return nil
}

func (r *PostgresSubmissionRepository) UpdateContainerInfo(ctx context.Context, id, containerIP string, containerPort int, containerID string) error {
	const q = `UPDATE submissions SET container_ip = $2, container_port = $3, container_id = $4 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, containerIP, containerPort, containerID)
	if err != nil {
		return fmt.Errorf("submissionRepo.UpdateContainerInfo: %w", err)
	}
	return nil
}
