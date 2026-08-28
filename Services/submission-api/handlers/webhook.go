package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/trade-eval/submission-api/apierrors"
	"github.com/trade-eval/submission-api/middleware"
	"github.com/trade-eval/submission-api/repository"
)

var allowedWebhookEvents = map[string]bool{
	"build_complete": true, "test_complete": true, "score_update": true, "anomaly": true,
}

type registerWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// CreateWebhook handles POST /v1/webhooks.
func (h *Handlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	contestant, ok := middleware.ContestantFromContext(r.Context())
	if !ok {
		apierrors.WriteError(w, &apierrors.ErrUnauthorized{Message: "no contestant in context"})
		return
	}
	if h.d.Webhooks == nil {
		apierrors.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webhooks unavailable"})
		return
	}
	var req registerWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "url", Message: "url is required"})
		return
	}
	for _, e := range req.Events {
		if !allowedWebhookEvents[e] {
			apierrors.WriteError(w, &apierrors.ErrValidation{Field: "events", Message: "unknown event: " + e})
			return
		}
	}
	if len(req.Events) == 0 {
		req.Events = []string{"build_complete", "test_complete"}
	}

	existing, _ := h.d.Webhooks.ListByContestant(r.Context(), contestant.ID)
	if len(existing) >= 10 {
		apierrors.WriteError(w, &apierrors.ErrConflict{Message: "webhook limit reached (10)"})
		return
	}

	var b [16]byte
	_, _ = rand.Read(b[:])
	wh := &repository.Webhook{
		ID:           "wh_" + uuid.Must(uuid.NewV7()).String(),
		ContestantID: contestant.ID,
		URL:          req.URL,
		Events:       req.Events,
		Secret:       hex.EncodeToString(b[:]),
	}
	if err := h.d.Webhooks.Create(r.Context(), wh); err != nil {
		apierrors.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create webhook"})
		return
	}
	apierrors.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": wh.ID, "url": wh.URL, "events": wh.Events, "secret": wh.Secret,
	})
}

// ListWebhooks handles GET /v1/webhooks.
func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	contestant, ok := middleware.ContestantFromContext(r.Context())
	if !ok || h.d.Webhooks == nil {
		apierrors.WriteJSON(w, http.StatusOK, map[string]any{"webhooks": []any{}})
		return
	}
	hooks, _ := h.d.Webhooks.ListByContestant(r.Context(), contestant.ID)
	out := make([]map[string]any, 0, len(hooks))
	for _, wh := range hooks {
		out = append(out, map[string]any{"id": wh.ID, "url": wh.URL, "events": wh.Events, "active": wh.Active})
	}
	apierrors.WriteJSON(w, http.StatusOK, map[string]any{"webhooks": out})
}
