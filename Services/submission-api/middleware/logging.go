package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// StructuredLogger logs each request as a structured slog record. It logs at
// ERROR for 5xx, WARN for 4xx, INFO otherwise, and emits a trace_id header.
func StructuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			reqID := middleware.GetReqID(r.Context())
			ww.Header().Set("X-Trace-Id", reqID)

			next.ServeHTTP(ww, r)

			status := ww.Status()
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", reqID,
			}
			if c, ok := ContestantFromContext(r.Context()); ok {
				attrs = append(attrs, "contestant_id", c.ID)
			}

			switch {
			case status >= 500:
				logger.Error("request", attrs...)
			case status >= 400:
				logger.Warn("request", attrs...)
			default:
				logger.Info("request", attrs...)
			}
		})
	}
}
