package openrouter

import (
	"context"
	"errors"

	"github.com/valpere/llm-council/internal/council"
)

var errNotImplemented = errors.New("openrouter: HTTP client not yet implemented")

// Client sends completion requests to the OpenRouter API.
// Full HTTP implementation is provided in a later milestone.
type Client struct {
	apiKey string
}

// NewClient creates a Client for the given OpenRouter API key.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

// Compile-time assertion: Client implements council.LLMClient.
var _ council.LLMClient = (*Client)(nil)

// Complete sends a chat completion request to OpenRouter.
// Stub — returns errNotImplemented until the HTTP layer is implemented in #77.
func (c *Client) Complete(_ context.Context, _ council.CompletionRequest) (council.CompletionResponse, error) {
	return council.CompletionResponse{}, errNotImplemented
}
