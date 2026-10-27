// Package generator is the HTTP client for the generator service.
package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/brunohms/cat-n-rat/app/backend/internal/purchase"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		// The instrumented transport propagates the trace context, so the generator spans join the same trace
		httpClient: &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

// FetchEvents asks the generator for new events.
func (c *Client) FetchEvents(ctx context.Context, count int) ([]purchase.Event, error) {
	// send=false so the generator does not also push these events back to us
	body, err := json.Marshal(map[string]any{"count": count, "send": false, "return_events": true})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/events/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("generator returned status %d: %s", resp.StatusCode, detail)
	}

	var payload struct {
		Events []purchase.Event `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode generator response: %w", err)
	}
	return payload.Events, nil
}
