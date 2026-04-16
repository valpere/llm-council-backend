package council

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var errNoChoices = errors.New("council: completion response contained no choices")

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

// RunFull orchestrates the full LCCP Core deliberation pipeline:
// label assignment → Stage 1 → quorum check → Stage 2 → Kendall's W → Stage 3.
// Events are emitted synchronously via onEvent so the caller can flush after each.
// Returns *QuorumError immediately if stage 1 quorum is not met (no stage2/3 events).
// Returns an error for unknown councilTypeName or stage 3 failures.
func (c *Council) RunFull(ctx context.Context, query string, councilTypeName string, onEvent EventFunc) error {
	ct, ok := c.registry[councilTypeName]
	if !ok {
		return fmt.Errorf("council: unknown council type %q", councilTypeName)
	}

	// Stage 1 — parallel generation across all configured models.
	allStage1 := c.runStage1(ctx, query, ct.Models, ct.Temperature)

	// Quorum check — returns *QuorumError if not enough models succeeded.
	successful, err := checkQuorum(allStage1, ct.QuorumMin)
	if err != nil {
		return err
	}

	// Assign anonymous labels so peer reviewers cannot identify each other.
	successfulModels := make([]string, len(successful))
	for i, r := range successful {
		successfulModels[i] = r.Model
	}
	labelToModel, modelToLabel := assignLabels(successfulModels)
	for i := range successful {
		successful[i].Label = modelToLabel[successful[i].Model]
	}

	if onEvent != nil {
		onEvent("stage1_complete", successful)
	}

	// Stage 2 — parallel peer review.
	stage2Results := c.runStage2(ctx, query, successful, ct.Temperature)

	// Compute aggregate rankings and Kendall's W consensus coefficient.
	allLabels := make([]string, 0, len(labelToModel))
	for label := range labelToModel {
		allLabels = append(allLabels, label)
	}
	aggregateRankings, consensusW := CalculateAggregateRankings(stage2Results, allLabels)

	metadata := Metadata{
		CouncilType:       councilTypeName,
		LabelToModel:      labelToModel,
		AggregateRankings: aggregateRankings,
		ConsensusW:        consensusW,
	}

	if onEvent != nil {
		onEvent("stage2_complete", Stage2CompleteData{Results: stage2Results, Metadata: metadata})
	}

	// Stage 3 — Chairman synthesis.
	labeledResponses := make(map[string]string, len(successful))
	for _, r := range successful {
		labeledResponses[r.Label] = r.Content
	}
	stage3Result, err := c.runStage3(ctx, query, stage2Results, labelToModel, consensusW, ct.ChairmanModel, ct.Temperature, labeledResponses)
	if err != nil {
		return err
	}

	if onEvent != nil {
		onEvent("stage3_complete", stage3Result)
	}

	return nil
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
			if err == nil && len(resp.Choices) == 0 {
				err = errNoChoices
			}
			results[i] = StageOneResult{
				Model:      model,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      err,
			}
			if err == nil {
				results[i].Content = resp.Choices[0].Message.Content
			}
		}(i, model)
	}
	wg.Wait()
	return results
}

// runStage2 sends peer-review requests to all stage1 models concurrently.
// Each reviewer receives the full set of anonymised stage1 responses and returns
// a ranked ordering as JSON. Parse failures are logged and treated as missing
// rankings so midrank imputation in CalculateAggregateRankings handles them.
// Unknown labels are logged and dropped from the ranking.
// LLM call failures are stored in StageTwoResult.Error; parse failures are not.
func (c *Council) runStage2(ctx context.Context, query string, stage1 []StageOneResult, temperature float64) []StageTwoResult {
	// Build the prompt and label maps once — shared across all reviewer goroutines.
	labeledResponses := make(map[string]string, len(stage1))
	knownLabels := make(map[string]bool, len(stage1))
	for _, r := range stage1 {
		labeledResponses[r.Label] = r.Content
		knownLabels[r.Label] = true
	}
	prompt := BuildStage2Prompt(query, labeledResponses)

	results := make([]StageTwoResult, len(stage1))
	var wg sync.WaitGroup
	for i, s1 := range stage1 {
		wg.Add(1)
		go func(i int, s1 StageOneResult) {
			defer wg.Done()
			resp, err := c.client.Complete(ctx, CompletionRequest{
				Model:          s1.Model,
				Messages:       prompt,
				Temperature:    temperature,
				ResponseFormat: &ResponseFormat{Type: "json_object"},
			})
			if err != nil {
				results[i] = StageTwoResult{ReviewerLabel: s1.Label, Error: err}
				return
			}
			if len(resp.Choices) == 0 {
				results[i] = StageTwoResult{ReviewerLabel: s1.Label, Error: errNoChoices}
				return
			}

			var parsed struct {
				Rankings []string `json:"rankings"`
			}
			if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &parsed); err != nil {
				if c.logger != nil {
					c.logger.Warn("stage2: parse failure", slog.String("reviewer", s1.Label), slog.Any("error", err))
				}
				results[i] = StageTwoResult{ReviewerLabel: s1.Label}
				return
			}
			if len(parsed.Rankings) == 0 {
				if c.logger != nil {
					c.logger.Warn("stage2: empty rankings", slog.String("reviewer", s1.Label))
				}
				results[i] = StageTwoResult{ReviewerLabel: s1.Label}
				return
			}

			valid := make([]string, 0, len(parsed.Rankings))
			for _, label := range parsed.Rankings {
				if knownLabels[label] {
					valid = append(valid, label)
				} else if c.logger != nil {
					c.logger.Warn("stage2: unknown label dropped", slog.String("reviewer", s1.Label), slog.String("label", label))
				}
			}
			results[i] = StageTwoResult{ReviewerLabel: s1.Label, Rankings: valid}
		}(i, s1)
	}
	wg.Wait()
	return results
}

// runStage3 calls the Chairman model to synthesize a final answer from the
// Stage 1 responses and Stage 2 peer-review rankings. Sequential — single LLM call.
func (c *Council) runStage3(ctx context.Context, query string, stage2 []StageTwoResult, labelToModel map[string]string, consensusW float64, chairmanModel string, temperature float64, labeledResponses map[string]string) (StageThreeResult, error) {
	start := time.Now()
	resp, err := c.client.Complete(ctx, CompletionRequest{
		Model:       chairmanModel,
		Messages:    BuildStage3Prompt(query, stage2, labelToModel, consensusW, labeledResponses),
		Temperature: temperature,
	})
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return StageThreeResult{Model: chairmanModel, DurationMs: elapsed}, fmt.Errorf("stage3: %w", err)
	}
	if len(resp.Choices) == 0 {
		return StageThreeResult{Model: chairmanModel, DurationMs: elapsed}, fmt.Errorf("stage3: %w", errNoChoices)
	}
	return StageThreeResult{
		Content:    resp.Choices[0].Message.Content,
		Model:      chairmanModel,
		DurationMs: elapsed,
	}, nil
}
