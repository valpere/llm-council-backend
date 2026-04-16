package council

import (
	"context"
	"errors"
	"log/slog"
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
