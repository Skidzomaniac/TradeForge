// Package client talks to contestant order-book containers over HTTP.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// OrderRequest is the POST /order body.
type OrderRequest struct {
	OrderID  string  `json:"order_id"`
	Type     string  `json:"type"`
	Price    float64 `json:"price,omitempty"`
	Quantity float64 `json:"quantity"`
}

// OrderResponse is the POST /order response.
type OrderResponse struct {
	OrderID           string  `json:"order_id"`
	Status            string  `json:"status"`
	FilledPrice       float64 `json:"filled_price"`
	FilledQuantity    float64 `json:"filled_quantity"`
	RemainingQuantity float64 `json:"remaining_quantity"`
}

// CancelResponse is the POST /cancel response.
type CancelResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// ErrTimedOut indicates the request exceeded its deadline.
var ErrTimedOut = errors.New("request timed out")

// OrderBookClient calls one contestant container.
type OrderBookClient struct {
	baseURL string
	http    *http.Client
	timeout time.Duration
}

// New constructs a client sharing the supplied transport.
func New(baseURL string, transport http.RoundTripper, timeout time.Duration) *OrderBookClient {
	return &OrderBookClient{
		baseURL: baseURL,
		http:    &http.Client{Transport: transport, Timeout: timeout},
		timeout: timeout,
	}
}

// SubmitOrder posts an order and measures round-trip latency.
func (c *OrderBookClient) SubmitOrder(ctx context.Context, order OrderRequest) (OrderResponse, time.Duration, error) {
	body, _ := json.Marshal(order)
	t1 := time.Now()
	resp, err := c.post(ctx, "/order", body)
	latency := time.Since(t1)
	if err != nil {
		if isTimeout(err) {
			return OrderResponse{}, c.timeout, ErrTimedOut
		}
		return OrderResponse{}, latency, fmt.Errorf("container_unreachable: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 500 {
		// A server error is a transport-level failure (counts against the breaker).
		return OrderResponse{Status: "REJECTED"}, latency, fmt.Errorf("server_error: status %d", resp.StatusCode)
	}
	var out OrderResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return OrderResponse{Status: "REJECTED"}, latency, nil
	}
	return out, latency, nil
}

// CancelOrder posts a cancel and measures latency.
func (c *OrderBookClient) CancelOrder(ctx context.Context, orderID string) (CancelResponse, time.Duration, error) {
	body, _ := json.Marshal(map[string]string{"order_id": orderID})
	t1 := time.Now()
	resp, err := c.post(ctx, "/cancel", body)
	latency := time.Since(t1)
	if err != nil {
		if isTimeout(err) {
			return CancelResponse{}, c.timeout, ErrTimedOut
		}
		return CancelResponse{}, latency, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 500 {
		return CancelResponse{}, latency, fmt.Errorf("server_error: status %d", resp.StatusCode)
	}
	var out CancelResponse
	_ = json.Unmarshal(data, &out)
	return out, latency, nil
}

// Reset calls POST /reset on the contestant container to clear all accumulated
// order book state so that each new test starts from a clean, empty book.
func (c *OrderBookClient) Reset(ctx context.Context) error {
	resp, err := c.post(ctx, "/reset", []byte("{}"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(io.LimitReader(resp.Body, 256))
	return nil
}

// HealthCheck returns true on a 200 from /health.
func (c *OrderBookClient) HealthCheck(ctx context.Context) bool {
	hctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isTimeout reports whether err is any flavour of request timeout: a context
// deadline, or an http.Client.Timeout (a net.Error with Timeout()==true).
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func (c *OrderBookClient) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}
