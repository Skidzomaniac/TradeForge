package repository

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/submission-api/apierrors"
)

// HashAPIKey returns the lowercase hex SHA-256 of an API key. It is the value
// stored in the contestants table and used as the Redis cache key, so the
// plaintext secret is never persisted or used as a key.
func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// ContestantRepository is the persistence contract for contestants.
type ContestantRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*Contestant, error)
	GetByID(ctx context.Context, id string) (*Contestant, error)
}

// PostgresContestantRepository implements ContestantRepository with a Redis
// read-through cache keyed by the API key hash (60s TTL).
type PostgresContestantRepository struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewPostgresContestantRepository constructs the repository.
func NewPostgresContestantRepository(pool *pgxpool.Pool, rdb *redis.Client) *PostgresContestantRepository {
	return &PostgresContestantRepository{pool: pool, redis: rdb}
}

func (r *PostgresContestantRepository) GetByAPIKey(ctx context.Context, apiKey string) (*Contestant, error) {
	hash := HashAPIKey(apiKey)
	cacheKey := "contestant_cache:" + hash
	if r.redis != nil {
		if cached, err := r.redis.Get(ctx, cacheKey).Result(); err == nil {
			var c Contestant
			if json.Unmarshal([]byte(cached), &c) == nil {
				return &c, nil
			}
		}
	}

	const q = `SELECT id, name, email, api_key_hash, created_at FROM contestants WHERE api_key_hash = $1`
	rows, err := r.pool.Query(ctx, q, hash)
	if err != nil {
		return nil, fmt.Errorf("contestantRepo.GetByAPIKey: %w", err)
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Contestant])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierrors.ErrUnauthorized{Message: "invalid API key"}
		}
		return nil, fmt.Errorf("contestantRepo.GetByAPIKey scan: %w", err)
	}
	// Constant-time check of the stored hash against the presented key's hash.
	if subtle.ConstantTimeCompare([]byte(c.APIKeyHash), []byte(hash)) != 1 {
		return nil, &apierrors.ErrUnauthorized{Message: "invalid API key"}
	}

	if r.redis != nil {
		if b, err := json.Marshal(c); err == nil {
			r.redis.Set(ctx, cacheKey, b, 60*time.Second)
		}
	}
	return c, nil
}

func (r *PostgresContestantRepository) GetByID(ctx context.Context, id string) (*Contestant, error) {
	const q = `SELECT id, name, email, api_key_hash, created_at FROM contestants WHERE id = $1`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("contestantRepo.GetByID: %w", err)
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Contestant])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierrors.ErrNotFound{Message: "contestant not found"}
		}
		return nil, fmt.Errorf("contestantRepo.GetByID scan: %w", err)
	}
	return c, nil
}
