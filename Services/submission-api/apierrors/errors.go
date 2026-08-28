// Package apierrors defines typed API errors and a consistent JSON writer.
package apierrors

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrNotFound is returned when a resource does not exist.
type ErrNotFound struct{ Message string }

func (e *ErrNotFound) Error() string { return e.Message }

// ErrConflict is returned when a request conflicts with current state.
type ErrConflict struct {
	Message string
	Details map[string]any
}

func (e *ErrConflict) Error() string { return e.Message }

// ErrValidation is returned when input fails validation.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string { return e.Message }

// ErrUnauthorized is returned when authentication fails.
type ErrUnauthorized struct{ Message string }

func (e *ErrUnauthorized) Error() string { return e.Message }

// ErrRateLimit is returned when a client exceeds its quota.
type ErrRateLimit struct {
	RetryAfter int // seconds
}

func (e *ErrRateLimit) Error() string { return "rate limit exceeded" }

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError maps a typed error to an HTTP status and writes a JSON body.
func WriteError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	msg := "internal server error"
	var details map[string]any

	var notFound *ErrNotFound
	var conflict *ErrConflict
	var validation *ErrValidation
	var unauthorized *ErrUnauthorized
	var rateLimit *ErrRateLimit

	switch {
	case errors.As(err, &notFound):
		status, code, msg = http.StatusNotFound, "NOT_FOUND", notFound.Message
	case errors.As(err, &conflict):
		status, code, msg = http.StatusConflict, "CONFLICT", conflict.Message
		details = conflict.Details
	case errors.As(err, &validation):
		status, code, msg = http.StatusBadRequest, "VALIDATION", validation.Message
		details = map[string]any{"field": validation.Field}
	case errors.As(err, &unauthorized):
		status, code, msg = http.StatusUnauthorized, "UNAUTHORIZED", unauthorized.Message
	case errors.As(err, &rateLimit):
		status, code, msg = http.StatusTooManyRequests, "RATE_LIMIT", "rate limit exceeded"
		w.Header().Set("Retry-After", itoa(rateLimit.RetryAfter))
	}

	WriteJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: msg, Details: details}})
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
