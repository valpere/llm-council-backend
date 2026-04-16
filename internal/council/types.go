package council

// Strategy identifies the deliberation algorithm used by a CouncilType.
type Strategy int

const (
	PeerReview Strategy = iota
)

// CouncilType describes a named council configuration.
// QuorumMin of 0 means use the formula: max(2, ⌈N/2⌉+1).
type CouncilType struct {
	Name          string
	Strategy      Strategy
	Models        []string
	ChairmanModel string
	Temperature   float64
	QuorumMin     int // 0 = use formula: max(2, ⌈N/2⌉+1)
}

// ChatMessage is a single turn in a conversation history.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat instructs the LLM to return a specific format.
type ResponseFormat struct {
	Type string `json:"type"` // e.g. "json_object"
}

// CompletionRequest is sent to the LLM gateway.
type CompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []ChatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// CompletionResponse is received from the LLM gateway.
type CompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// EventFunc is the callback used to stream stage-completion events to the caller.
type EventFunc func(eventType string, data any)

// StageOneResult holds a single council member's generated answer.
type StageOneResult struct {
	Label      string `json:"label"`      // anonymised label, e.g. "Response A"
	Content    string `json:"content"`
	Model      string `json:"model"`
	DurationMs int64  `json:"duration_ms"` // elapsed milliseconds
	Error      error  `json:"-"`
}

// StageTwoResult holds a single council member's peer-review rankings.
type StageTwoResult struct {
	ReviewerLabel string   `json:"reviewer_label"`
	Rankings      []string `json:"rankings"` // ordered labels, best first
	Error         error    `json:"-"`
}

// StageThreeResult holds the chairman's synthesised final answer.
type StageThreeResult struct {
	Content    string `json:"content"`
	Model      string `json:"model"`
	DurationMs int64  `json:"duration_ms"` // elapsed milliseconds
	Error      error  `json:"-"`
}

// RankedModel pairs a model name with its aggregate rank score.
type RankedModel struct {
	Model string  `json:"model"`
	Score float64 `json:"score"`
}

// Metadata is persisted with every assistant message.
type Metadata struct {
	CouncilType       string        `json:"council_type"`
	LabelToModel      map[string]string `json:"label_to_model"`
	AggregateRankings []RankedModel `json:"aggregate_rankings"`
	ConsensusW        float64       `json:"consensus_w"`
}

// Stage2CompleteData is the payload emitted by Runner for the "stage2_complete" event.
// It bundles peer-review results with the computed aggregate metadata so callers
// (e.g. the SSE handler) can surface both in one event.
type Stage2CompleteData struct {
	Results  []StageTwoResult `json:"results"`
	Metadata Metadata         `json:"metadata"`
}

// AssistantMessage is the full deliberation record stored with each assistant turn.
type AssistantMessage struct {
	Role     string           `json:"role"`
	Stage1   []StageOneResult `json:"stage1"`
	Stage2   []StageTwoResult `json:"stage2"`
	Stage3   StageThreeResult `json:"stage3"`
	Metadata Metadata         `json:"metadata"`
}
