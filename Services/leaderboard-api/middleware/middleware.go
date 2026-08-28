// Package middleware provides CORS and Redis-backed rate limiting.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// CORS allows cross-origin reads (the leaderboard is public).
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimit enforces 60 req/min/IP via a Redis sliding-window counter.
func RateLimit(rdb *redis.Client, limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rdb == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			bucket := time.Now().Unix() / 60
			key := "ratelimit:" + r.URL.Path + ":" + ip + ":" + strconv.FormatInt(bucket, 10)
			count, err := rdb.Incr(r.Context(), key).Result()
			if err == nil {
				if count == 1 {
					rdb.Expire(r.Context(), key, 120*time.Second)
				}
				if int(count) > limit {
					w.Header().Set("Retry-After", "60")
					http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
