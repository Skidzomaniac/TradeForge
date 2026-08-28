// Package webhooks delivers signed event callbacks to contestant endpoints.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/trade-eval/submission-api/repository"
)

// Manager dispatches HMAC-signed webhook deliveries with bounded retries.
type Manager struct {
	repo   repository.WebhookRepository
	client *http.Client
	logger *slog.Logger
}

// NewManager constructs the manager.
func NewManager(repo repository.WebhookRepository, logger *slog.Logger) *Manager {
	return &Manager{repo: repo, client: &http.Client{Timeout: 5 * time.Second}, logger: logger}
}

// Fire delivers an event to all of a contestant's matching webhooks (async).
func (m *Manager) Fire(contestantID, event string, payload any) {
	hooks, err := m.repo.ListForEvent(context.Background(), contestantID, event)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(map[string]any{"event": event, "data": payload, "sent_at": time.Now().Unix()})
	if err != nil {
		return
	}
	for _, h := range hooks {
		go m.deliver(h, event, body)
	}
}

func (m *Manager) deliver(h *repository.Webhook, event string, body []byte) {
	sig := sign(h.Secret, body)
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, h.URL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-TradeEval-Event", event)
		req.Header.Set("X-TradeEval-Signature", "sha256="+sig)
		resp, err := m.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(time.Duration(attempt*attempt) * time.Second)
	}
	m.logger.Warn("webhook delivery failed", "url", h.URL, "event", event)
}

// sign returns the hex HMAC-SHA256 of body keyed by secret.
func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
