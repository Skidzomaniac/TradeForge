// Package middleware contains HTTP middleware for submission-api.
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/submission-api/apierrors"
	"github.com/trade-eval/submission-api/repository"
)

// rateLimitScript increments a counter and sets its expiry on first use in a
// single atomic step, so the key can never be left without a TTL (which would
// rate-limit a contestant forever). It returns the post-increment count.
var rateLimitScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return c
`)

const submissionRateLimitPerHour = 10

type ctxKey string

// ContestantContextKey is the context key under which the authenticated
// contestant is stored.
const ContestantContextKey ctxKey = "contestant"

// ContestantFromContext returns the authenticated contestant, if any.
func ContestantFromContext(ctx context.Context) (*repository.Contestant, bool) {
	c, ok := ctx.Value(ContestantContextKey).(*repository.Contestant)
	return c, ok
}

// Auth builds the API-key authentication + per-contestant rate-limiting
// middleware. Rate limit: 10 submissions/hour, enforced only on POST
// /v1/submissions.
func Auth(repo repository.ContestantRepository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apierrors.WriteError(w, &apierrors.ErrUnauthorized{Message: "missing X-API-Key header"})
				return
			}

			contestant, err := repo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				apierrors.WriteError(w, err)
				return
			}

			// Rate limit submission creation per contestant. The increment and the
			// expiry are applied atomically so a crash between them cannot leave a
			// key without a TTL.
			if r.Method == http.MethodPost && r.URL.Path == "/v1/submissions" && rdb != nil {
				key := "rate_limit:submission:" + contestant.ID
				count, err := rateLimitScript.Run(r.Context(), rdb, []string{key}, int(time.Hour.Seconds())).Int64()
				if err == nil && count > submissionRateLimitPerHour {
					apierrors.WriteError(w, &apierrors.ErrRateLimit{RetryAfter: 3600})
					return
				}
			}

			ctx := context.WithValue(r.Context(), ContestantContextKey, contestant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
