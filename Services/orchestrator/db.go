package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRow is a minimal projection of the tests table used by the orchestrator.
type TestRow struct {
	ID             string
	SubmissionID   string
	ContestantID   string
	Status         string
	StartedAt      *time.Time
	DurationSec    int
	LastHeartbeat  *time.Time
	InstanceID     *string
}

// TestRepo wraps the orchestrator's Postgres access.
type TestRepo struct {
	pool *pgxpool.Pool
}

// NewTestRepo constructs the repository.
func NewTestRepo(pool *pgxpool.Pool) *TestRepo { return &TestRepo{pool: pool} }

// CASStatus performs an optimistic status transition. Returns true if a row
// was updated (i.e., the expected current status matched).
func (r *TestRepo) CASStatus(ctx context.Context, testID, from, to, instanceID string) (bool, error) {
	const q = `
		UPDATE tests
		SET status = $3, updated_at = NOW(), orchestrator_instance_id = $4
		WHERE id = $1 AND status = $2`
	tag, err := r.pool.Exec(ctx, q, testID, from, to, instanceID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// SetFailureReason records why a test failed.
func (r *TestRepo) SetFailureReason(ctx context.Context, testID, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE tests SET failure_reason = $2 WHERE id = $1`, testID, reason)
	return err
}

// MarkEnded stamps ended_at and final_score on completion.
func (r *TestRepo) MarkEnded(ctx context.Context, testID string, score float64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET ended_at = NOW(), final_score = $2 WHERE id = $1`, testID, score)
	return err
}

// CurrentStatus returns the current status of a test.
func (r *TestRepo) CurrentStatus(ctx context.Context, testID string) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM tests WHERE id = $1`, testID).Scan(&status)
	return status, err
}

// Heartbeat updates last_heartbeat_at for tests this instance owns.
func (r *TestRepo) Heartbeat(ctx context.Context, testID, instanceID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET last_heartbeat_at = NOW() WHERE id = $1 AND orchestrator_instance_id = $2`,
		testID, instanceID)
	return err
}

// FindOrphans returns running/stopping tests with stale heartbeats.
func (r *TestRepo) FindOrphans(ctx context.Context, staleSeconds int) ([]TestRow, error) {
	const q = `
		SELECT id, submission_id, contestant_id, status, started_at, last_heartbeat_at, orchestrator_instance_id
		FROM tests
		WHERE status IN ('running','stopping')
		  AND (last_heartbeat_at IS NULL OR last_heartbeat_at < NOW() - make_interval(secs => $1))`
	rows, err := r.pool.Query(ctx, q, staleSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRow
	for rows.Next() {
		var t TestRow
		if err := rows.Scan(&t.ID, &t.SubmissionID, &t.ContestantID, &t.Status, &t.StartedAt, &t.LastHeartbeat, &t.InstanceID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimRecovery sets the owning instance for a recovered test.
func (r *TestRepo) ClaimRecovery(ctx context.Context, testID, instanceID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET orchestrator_instance_id = $2, last_heartbeat_at = NOW() WHERE id = $1`,
		testID, instanceID)
	return err
}

// GetContestantName looks up a contestant's display name. Returns empty string on any error.
func (r *TestRepo) GetContestantName(ctx context.Context, contestantID string) string {
	var name string
	_ = r.pool.QueryRow(ctx, `SELECT name FROM contestants WHERE id = $1`, contestantID).Scan(&name)
	return name
}
