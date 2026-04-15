package council

import "context"

// LLMClient is the interface for sending completion requests to an LLM gateway.
// openrouter.Client implements this; council logic depends only on this interface.
type LLMClient interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// Runner orchestrates a full council deliberation.
// All stage results are delivered via onEvent — the caller never receives
// stage structs directly, keeping the handler free of council-internal types.
// councilType is a string name resolved to a CouncilType by the Runner implementation.
type Runner interface {
	RunFull(ctx context.Context, query string, councilType string, onEvent EventFunc) error
}
