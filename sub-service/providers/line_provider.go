package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type LINEProvider struct {
	endpoint      string
	httpClient    *http.Client
	retryAttempts int
	retryDelay    time.Duration
}

type LINEMessageRequest struct {
	To       string        `json:"to"`
	Messages []LINEMessage `json:"messages"`
}

type LINEMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewLINEProvider() (*LINEProvider, error) {
	endpoint := os.Getenv("LINE_API_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("LINE_API_ENDPOINT is required")
	}

	retryAttempts := 3
	if ra := os.Getenv("LINE_RETRY_ATTEMPTS"); ra != "" {
		var parsed int
		if _, err := fmt.Sscanf(ra, "%d", &parsed); err == nil && parsed > 0 {
			retryAttempts = parsed
		}
	}

	retryDelay := 1000 * time.Millisecond
	if rd := os.Getenv("LINE_RETRY_DELAY"); rd != "" {
		var parsed int
		if _, err := fmt.Sscanf(rd, "%d", &parsed); err == nil && parsed > 0 {
			retryDelay = time.Duration(parsed) * time.Millisecond
		}
	}

	return &LINEProvider{
		endpoint:      endpoint,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		retryAttempts: retryAttempts,
		retryDelay:    retryDelay,
	}, nil
}

func (p *LINEProvider) PushMessage(ctx context.Context, to string, message string) error {
	req := LINEMessageRequest{
		To: to,
		Messages: []LINEMessage{
			{
				Type: "text",
				Text: message,
			},
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal LINE message: %w", err)
	}

	var lastErr error
	for i := 0; i < p.retryAttempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.retryDelay):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/v2/bot/message/push", bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create HTTP request: %w", err)
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"))

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("failed to send HTTP request (attempt %d): %w", i+1, err)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		lastErr = fmt.Errorf("LINE API returned status %d (attempt %d)", resp.StatusCode, i+1)
		resp.Body.Close()
	}

	return lastErr
}

func (p *LINEProvider) GetEndpoint() string {
	return p.endpoint
}
