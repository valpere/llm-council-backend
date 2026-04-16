package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/valpere/llm-council/internal/council"
)

const (
	defaultURL   = "https://openrouter.ai/api/v1/chat/completions"
	maxBodyBytes = 4 * 1024 * 1024 // 4 MiB cap on response bodies
)

// APIError is returned when OpenRouter responds with a non-200 status code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openrouter: API error %d: %s", e.StatusCode, e.Body)
}

// Client sends completion requests to the OpenRouter API.
type Client struct {
	apiKey  string
	baseURL string // overridable in tests; defaults to defaultURL
	http    *http.Client
}

// NewClient creates a Client with the given API key and HTTP timeout.
func NewClient(apiKey string, timeout time.Duration) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// Compile-time assertion: Client implements council.LLMClient.
var _ council.LLMClient = (*Client)(nil)

// Complete POSTs a chat completion request to OpenRouter and returns the response.
// Returns *APIError on non-200 responses.
func (c *Client) Complete(ctx context.Context, req council.CompletionRequest) (council.CompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/valpere/llm-council")
	httpReq.Header.Set("X-Title", "LLM Council")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Limit reads to maxBodyBytes to guard against unexpectedly large responses.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return council.CompletionResponse{}, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var completionResp council.CompletionResponse
	if err := json.Unmarshal(respBody, &completionResp); err != nil {
		return council.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	return completionResp, nil
}
