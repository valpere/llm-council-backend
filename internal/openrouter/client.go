package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const apiURL = "https://openrouter.ai/api/v1/chat/completions"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		// 5-minute transport-level ceiling acts as a safety backstop against a hung
		// connection. Per-request deadlines via context.WithTimeout (120 s) should
		// normally fire first; this timeout only applies if something goes badly wrong.
		httpClient: &http.Client{Timeout: 300 * time.Second},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Content string
}

type ModelResult struct {
	Model    string
	Response *Response
	Err      error
}

func (c *Client) QueryModel(ctx context.Context, model string, messages []Message, timeout time.Duration) (*Response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10)) // cap at 64KB
		return nil, fmt.Errorf("openrouter error %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response from %s", model)
	}

	return &Response{Content: result.Choices[0].Message.Content}, nil
}

func (c *Client) QueryModelsParallel(ctx context.Context, models []string, messages []Message, timeout time.Duration) []ModelResult {
	results := make([]ModelResult, len(models))
	var wg sync.WaitGroup
	for i, model := range models {
		wg.Add(1)
		go func(i int, model string) {
			defer wg.Done()
			resp, err := c.QueryModel(ctx, model, messages, timeout)
			results[i] = ModelResult{Model: model, Response: resp, Err: err}
		}(i, model)
	}
	wg.Wait()
	return results
}
