package council

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var errNotImplemented = errors.New("council: RunFull not yet implemented")

// Council orchestrates the full multi-stage deliberation pipeline.
// Full implementation is provided in a later milestone.
type Council struct {
	client   LLMClient
	registry map[string]CouncilType
	logger   *slog.Logger
}

// NewCouncil creates a Council that uses client for LLM calls and resolves
// named council types from registry.
func NewCouncil(client LLMClient, registry map[string]CouncilType, logger *slog.Logger) *Council {
	return &Council{
		client:   client,
		registry: registry,
		logger:   logger,
	}
}

// Compile-time assertion: Council implements Runner.
var _ Runner = (*Council)(nil)

// RunFull orchestrates a full council deliberation.
// Stub — returns errNotImplemented until #86 is implemented.
func (c *Council) RunFull(_ context.Context, _ string, _ string, _ EventFunc) error {
	return errNotImplemented
}

// runStage1 sends query to all models concurrently and returns all results.
// Each goroutine writes to its own pre-allocated results[i] slot — no mutex needed.
// Context cancellation is propagated to every Complete call.
// Quorum is NOT checked here — that is the caller's responsibility.
func (c *Council) runStage1(ctx context.Context, query string, models []string, temperature float64) []StageOneResult {
	results := make([]StageOneResult, len(models))
	var wg sync.WaitGroup
	for i, model := range models {
		wg.Add(1)
		go func(i int, model string) {
			defer wg.Done()
			start := time.Now()
			resp, err := c.client.Complete(ctx, CompletionRequest{
				Model:       model,
				Messages:    BuildStage1Prompt(query),
				Temperature: temperature,
			})
			results[i] = StageOneResult{
				Model:      model,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      err,
			}
			if err == nil && len(resp.Choices) > 0 {
				results[i].Content = resp.Choices[0].Message.Content
			}
		}(i, model)
	}
	wg.Wait()
	return results
}
