package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/trade-eval/submission-api/apierrors"
	"github.com/trade-eval/submission-api/middleware"
	"github.com/trade-eval/submission-api/repository"
	"github.com/trade-eval/submission-api/security"
)

type createTestRequest struct {
	SubmissionID    string   `json:"submission_id"`
	DurationSeconds int      `json:"duration_seconds"`
	BotCount        int      `json:"bot_count"`
	BotPersonas     []string `json:"bot_personas"`
}

type startTestEvent struct {
	Event           string   `json:"event"`
	TestID          string   `json:"test_id"`
	ContestantID    string   `json:"contestant_id"`
	TargetIP        string   `json:"target_ip"`
	TargetPort      int      `json:"target_port"`
	DurationSeconds int      `json:"duration_seconds"`
	BotCount        int      `json:"bot_count"`
	BotPersonas     []string `json:"bot_personas"`
}

// CreateTest handles POST /v1/tests.
func (h *Handlers) CreateTest(w http.ResponseWriter, r *http.Request) {
	contestant, ok := middleware.ContestantFromContext(r.Context())
	if !ok {
		apierrors.WriteError(w, &apierrors.ErrUnauthorized{Message: "no contestant in context"})
		return
	}

	var req createTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "body", Message: "invalid JSON body"})
		return
	}
	if req.SubmissionID == "" {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "submission_id", Message: "submission_id is required"})
		return
	}
	if err := security.ValidateTestParams(req.DurationSeconds, req.BotCount, req.BotPersonas); err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 300
	}
	if req.BotCount == 0 {
		req.BotCount = 500
	}
	if len(req.BotPersonas) == 0 {
		req.BotPersonas = []string{"market_maker", "aggressive_taker", "spammer", "whale"}
	}

	// 1. Validate submission ownership.
	sub, err := h.d.Submissions.GetByID(r.Context(), req.SubmissionID)
	if err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if sub.ContestantID != contestant.ID {
		apierrors.WriteError(w, &apierrors.ErrNotFound{Message: "submission not found"})
		return
	}

	// 2. Submission must be ready.
	if sub.Status != "ready" {
		apierrors.WriteError(w, &apierrors.ErrConflict{
			Message: "submission is not ready",
			Details: map[string]any{"status": sub.Status},
		})
		return
	}

	// 3. No active test for this contestant.
	if active, err := h.d.Tests.GetActiveByContestantID(r.Context(), contestant.ID); err == nil && active != nil {
		apierrors.WriteError(w, &apierrors.ErrConflict{
			Message: "test already running",
			Details: map[string]any{"test_id": active.ID},
		})
		return
	} else if err != nil {
		var nf *apierrors.ErrNotFound
		if !errors.As(err, &nf) {
			apierrors.WriteError(w, err)
			return
		}
	}

	// 4. Create test row.
	testID := "test_" + uuid.Must(uuid.NewV7()).String()
	test := &repository.Test{
		ID:           testID,
		SubmissionID: sub.ID,
		ContestantID: contestant.ID,
		Status:       "pending",
	}
	if err := h.d.Tests.Create(r.Context(), test); err != nil {
		apierrors.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create test"})
		return
	}

	// 6. Publish START_TEST.
	ip, port := "", 0
	if sub.ContainerIP != nil {
		ip = *sub.ContainerIP
	}
	if sub.ContainerPort != nil {
		port = *sub.ContainerPort
	}
	evt := startTestEvent{
		Event:           "START_TEST",
		TestID:          testID,
		ContestantID:    contestant.ID,
		TargetIP:        ip,
		TargetPort:      port,
		DurationSeconds: req.DurationSeconds,
		BotCount:        req.BotCount,
		BotPersonas:     req.BotPersonas,
	}
	if b, err := json.Marshal(evt); err == nil && h.d.Producer != nil {
		if err := h.d.Producer.Produce(r.Context(), h.d.OrchEventsTopic, []byte(contestant.ID), b); err != nil {
			h.d.Logger.Warn("failed to publish START_TEST", "error", err)
		}
	}

	apierrors.WriteJSON(w, http.StatusAccepted, map[string]string{"test_id": testID, "status": "pending"})
}

// GetTest handles GET /v1/tests/{id}, merging live Redis metrics.
func (h *Handlers) GetTest(w http.ResponseWriter, r *http.Request) {
	contestant, _ := middleware.ContestantFromContext(r.Context())
	id := chi.URLParam(r, "id")
	test, err := h.d.Tests.GetByID(r.Context(), id)
	if err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if contestant == nil || test.ContestantID != contestant.ID {
		apierrors.WriteError(w, &apierrors.ErrNotFound{Message: "test not found"})
		return
	}

	live := map[string]string{}
	if h.d.Redis != nil {
		if m, err := h.d.Redis.HGetAll(r.Context(), "metrics:"+test.ContestantID).Result(); err == nil {
			for _, k := range []string{"p50_latency_us", "p90_latency_us", "p99_latency_us", "tps", "correctness_rate"} {
				if v, ok := m[k]; ok {
					live[k] = v
				}
			}
		}
	}

	apierrors.WriteJSON(w, http.StatusOK, map[string]any{
		"test":         test,
		"live_metrics": live,
	})
}
