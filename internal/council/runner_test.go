package council

import (
	"context"
	"errors"
	"testing"
)

// mockLLMClient implements LLMClient for testing.
type mockLLMClient struct {
	complete func(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

func (m *mockLLMClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	return m.complete(ctx, req)
}

func makeResponse(content string) CompletionResponse {
	return CompletionResponse{
		Choices: []struct {
			Message ChatMessage `json:"message"`
		}{
			{Message: ChatMessage{Role: "assistant", Content: content}},
		},
	}
}

// ── runStage1 ─────────────────────────────────────────────────────────────────

func TestRunStage1_AllSucceed(t *testing.T) {
	models := []string{"model-a", "model-b", "model-c"}
	client := &mockLLMClient{
		complete: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			return makeResponse("answer from " + req.Model), nil
		},
	}
	c := NewCouncil(client, nil, nil)
	results := c.runStage1(context.Background(), "test query", models, 0.7)

	if len(results) != len(models) {
		t.Fatalf("len: got %d, want %d", len(results), len(models))
	}
	for i, r := range results {
		if r.Error != nil {
			t.Errorf("results[%d].Error: unexpected %v", i, r.Error)
		}
		if r.Content == "" {
			t.Errorf("results[%d].Content: empty", i)
		}
		if r.Model != models[i] {
			t.Errorf("results[%d].Model: got %q, want %q", i, r.Model, models[i])
		}
		if r.DurationMs < 0 {
			t.Errorf("results[%d].DurationMs: negative %d", i, r.DurationMs)
		}
	}
}

func TestRunStage1_PartialFailure_ReturnsAll(t *testing.T) {
	errBoom := errors.New("model failed")
	models := []string{"model-a", "model-b", "model-c"}
	client := &mockLLMClient{
		complete: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			if req.Model == "model-b" {
				return CompletionResponse{}, errBoom
			}
			return makeResponse("ok"), nil
		},
	}
	c := NewCouncil(client, nil, nil)
	results := c.runStage1(context.Background(), "test query", models, 0.7)

	if len(results) != 3 {
		t.Fatalf("len: got %d, want 3", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("results[0]: unexpected error %v", results[0].Error)
	}
	if !errors.Is(results[1].Error, errBoom) {
		t.Errorf("results[1].Error: got %v, want errBoom", results[1].Error)
	}
	if results[1].Content != "" {
		t.Errorf("results[1].Content: want empty on error, got %q", results[1].Content)
	}
	if results[2].Error != nil {
		t.Errorf("results[2]: unexpected error %v", results[2].Error)
	}
}

func TestRunStage1_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before any call

	client := &mockLLMClient{
		complete: func(ctx context.Context, _ CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, ctx.Err()
		},
	}
	c := NewCouncil(client, nil, nil)
	results := c.runStage1(ctx, "test query", []string{"model-a", "model-b"}, 0.7)

	if len(results) != 2 {
		t.Fatalf("len: got %d, want 2", len(results))
	}
	for i, r := range results {
		if !errors.Is(r.Error, context.Canceled) {
			t.Errorf("results[%d].Error: got %v, want context.Canceled", i, r.Error)
		}
	}
}

func TestRunStage1_EmptyChoices_ContentEmpty(t *testing.T) {
	client := &mockLLMClient{
		complete: func(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, nil // no choices
		},
	}
	c := NewCouncil(client, nil, nil)
	results := c.runStage1(context.Background(), "q", []string{"model-a"}, 0.7)

	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
	if results[0].Content != "" {
		t.Errorf("Content: want empty when no choices, got %q", results[0].Content)
	}
}
