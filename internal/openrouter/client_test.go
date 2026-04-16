package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/valpere/llm-council/internal/council"
)

// testClient returns a Client pointed at srv with the given API key.
func testClient(apiKey string, srv *httptest.Server) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: srv.URL,
		http:    srv.Client(),
	}
}

// mockCompletion is the minimal OpenAI-compatible completion shape.
type mockCompletion struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ── TestComplete_RequiredHeaders ──────────────────────────────────────────────

func TestComplete_RequiredHeaders(t *testing.T) {
	var gotAuth, gotReferer, gotTitle string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		writeJSON(w, mockCompletion{
			Choices: []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "hi"}},
			},
		})
	}))
	defer srv.Close()

	c := testClient("sk-test-key", srv)
	_, err := c.Complete(context.Background(), council.CompletionRequest{
		Model:    "openai/gpt-4o-mini",
		Messages: []council.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer sk-test-key" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer sk-test-key")
	}
	if gotReferer == "" {
		t.Error("HTTP-Referer header missing")
	}
	if gotTitle == "" {
		t.Error("X-Title header missing")
	}
}

// ── TestComplete_SuccessfulResponse ──────────────────────────────────────────

func TestComplete_SuccessfulResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, mockCompletion{
			Choices: []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{Role: "assistant", Content: "Paris"}},
			},
		})
	}))
	defer srv.Close()

	c := testClient("key", srv)
	resp, err := c.Complete(context.Background(), council.CompletionRequest{
		Model:    "openai/gpt-4o-mini",
		Messages: []council.ChatMessage{{Role: "user", Content: "capital of France?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("no choices in response")
	}
	if got := resp.Choices[0].Message.Content; got != "Paris" {
		t.Errorf("content: got %q, want %q", got, "Paris")
	}
}

// ── TestComplete_APIError_OnNonOK ─────────────────────────────────────────────

func TestComplete_APIError_OnNonOK(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 bad request", http.StatusBadRequest},
		{"429 rate limited", http.StatusTooManyRequests},
		{"500 internal error", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"error":"test error"}`))
			}))
			defer srv.Close()

			c := testClient("key", srv)
			_, err := c.Complete(context.Background(), council.CompletionRequest{
				Model:    "openai/gpt-4o-mini",
				Messages: []council.ChatMessage{{Role: "user", Content: "hi"}},
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.statusCode {
				t.Errorf("StatusCode: got %d, want %d", apiErr.StatusCode, tc.statusCode)
			}
			if apiErr.Body == "" {
				t.Error("APIError.Body should not be empty")
			}
		})
	}
}

// ── TestComplete_ResponseFormatForwarded ─────────────────────────────────────

func TestComplete_ResponseFormatForwarded(t *testing.T) {
	tests := []struct {
		name           string
		responseFormat *council.ResponseFormat
		wantInBody     bool
	}{
		{
			name:           "nil response_format omitted from request",
			responseFormat: nil,
			wantInBody:     false,
		},
		{
			name:           "json_object format forwarded",
			responseFormat: &council.ResponseFormat{Type: "json_object"},
			wantInBody:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				writeJSON(w, mockCompletion{
					Choices: []struct {
						Message struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						} `json:"message"`
					}{
						{Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{Role: "assistant", Content: "{}"}},
					},
				})
			}))
			defer srv.Close()

			c := testClient("key", srv)
			_, err := c.Complete(context.Background(), council.CompletionRequest{
				Model:          "openai/gpt-4o-mini",
				Messages:       []council.ChatMessage{{Role: "user", Content: "hi"}},
				ResponseFormat: tc.responseFormat,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_, present := gotBody["response_format"]
			if present != tc.wantInBody {
				t.Errorf("response_format present=%v, want %v", present, tc.wantInBody)
			}
		})
	}
}

// ── TestNewClient ─────────────────────────────────────────────────────────────

func TestNewClient(t *testing.T) {
	c := NewClient("my-key", 30*time.Second)
	if c.apiKey != "my-key" {
		t.Errorf("apiKey: got %q, want %q", c.apiKey, "my-key")
	}
	if c.baseURL != defaultURL {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, defaultURL)
	}
	if c.http.Timeout != 30*time.Second {
		t.Errorf("timeout: got %v, want 30s", c.http.Timeout)
	}
}
