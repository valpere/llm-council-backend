> **v1 reference — archived.** This document describes the implementation on `archive/v1`.
> For the active v2 planning documents see [`council-research-synthesis.md`](council-research-synthesis.md) and [`council-research-gaps.md`](council-research-gaps.md).

# Go Implementation Notes

The original Python/FastAPI backend has been rewritten in Go. The frontend (React + Vite) lives in the `frontend/` directory.

## Package Structure

```
llm-council/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, config validation, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, Load() from env, Validate()
│   ├── openrouter/
│   │   └── client.go        # QueryModel(), QueryModelsParallel()
│   ├── council/
│   │   ├── interfaces.go    # LLMClient and Runner interfaces; compile-time checks
│   │   ├── types.go         # StageOneResult, StageTwoResult, AggregateRanking, Metadata, Result
│   │   ├── prompts.go       # rankingPromptTemplate, chairmanPromptTemplate, titlePromptTemplate
│   │   ├── council.go       # Council struct; stage functions; CalculateAggregateRankings; kendallW
│   │   └── council_test.go  # Unit tests for parseRankingFromText, CalculateAggregateRankings
│   ├── storage/
│   │   ├── storage.go       # Storer interface; Store struct; Create/Get/AddMessage/UpdateTitle/List
│   │   └── storage_test.go  # Integration tests with real filesystem; race-detection coverage
│   └── api/
│       ├── handler.go       # HTTP handlers, CORS middleware, SSE streaming
│       └── handler_test.go  # Handler tests using fakeCouncil / fakeStore stubs
├── data/
│   └── conversations/       # JSON conversation files
├── tools.go                 # //go:build tools — pins staticcheck for go run
├── go.mod
├── go.sum
└── .env                     # Local secrets (not committed)
```

Frontend lives in the `frontend/` directory.

## Key Implementation Notes

### Concurrency

Parallel model queries use `sync.WaitGroup` with per-goroutine results:

```go
results := make([]ModelResult, len(models))
var wg sync.WaitGroup
for i, model := range models {
    wg.Add(1)
    go func(i int, model string) {
        defer wg.Done()
        resp, err := client.QueryModel(ctx, model, messages, timeout)
        results[i] = ModelResult{Model: model, Response: resp, Err: err}
    }(i, model)
}
wg.Wait()
```

### SSE Streaming

Stage completion events are sent over a `text/event-stream` response. Each event is a single `data:` line containing a JSON object with a `type` field:

```
data: {"type":"stage1_start"}

data: {"type":"stage1_complete","data":[...]}

data: {"type":"stage2_complete","data":[...],"metadata":{"label_to_model":{...},"aggregate_rankings":[...],"consensus_w":0.72}}

data: {"type":"stage3_complete","data":{...}}

data: {"type":"title_complete","data":{"title":"..."}}

data: {"type":"complete"}
```

### Interfaces

Two key interfaces defined in `internal/council/interfaces.go` enable testing without real LLM or storage calls:

```go
// LLMClient — implemented by openrouter.Client; mock it in tests.
type LLMClient interface {
    QueryModel(ctx, model, messages, timeout) (*Response, error)
    QueryModelsParallel(ctx, models, messages, timeout) []ModelResult
}

// Runner — implemented by Council; inject a mock to test handlers.
type Runner interface {
    Stage1CollectResponses(ctx, query) ([]StageOneResult, error)
    Stage2CollectRankings(ctx, query, stage1) ([]StageTwoResult, map[string]string, error)
    Stage3SynthesizeFinal(ctx, query, stage1, stage2 []StageTwoResult, consensusW float64) (StageThreeResult, error)
    GenerateTitle(ctx, query) string
    RunFull(ctx, query) (Result, error)
    CalculateAggregateRankings(stage2, labelToModel) ([]AggregateRanking, float64)
}
```

`storage.Storer` (defined in `internal/storage/storage.go`) does the same for the persistence layer.

Compile-time assertions in `interfaces.go` ensure concrete types satisfy their interfaces:
```go
var _ Runner = (*Council)(nil)
var _ LLMClient = (*openrouter.Client)(nil)
```

### Configuration

Loaded from environment variables (`.env` via `godotenv`). Validated at startup — the process exits immediately if `OPENROUTER_API_KEY` is missing.

```go
type Config struct {
    OpenRouterAPIKey string
    CouncilModels    []string   // COUNCIL_MODELS — comma-separated; default: 4 models
    ChairmanModel    string     // CHAIRMAN_MODEL — default: gemini-3-pro-preview
    TitleModel       string     // TITLE_MODEL — default: gemini-2.5-flash
    DataDir          string     // DATA_DIR — default: data/conversations
    Port             string     // PORT — default: 8001
    CORSOrigins      []string   // CORS_ORIGINS — comma-separated; default: localhost:5173,localhost:3000
}
```

### Storage

Each conversation is a single JSON file at `data/conversations/{uuid}.json`.

- Writes are atomic: data is written to a `.tmp` file then renamed, preventing partial writes on crash.
- Concurrent writes to the same conversation are serialized via a per-conversation `sync.Mutex`.
- Conversation IDs are validated against a UUID regex before any file path is constructed, preventing directory traversal.

### Title Generation

Title generation runs in a goroutine started before `RunFull()` so it overlaps with the council pipeline. It only runs for the first message in a conversation (`isFirst`). The result is captured through a closure (`awaitTitle`) that blocks on the channel, keeping the channel scoped to the `if` block:

```go
var awaitTitle func() string
if isFirst {
    ch := make(chan string, 1)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        ch <- h.council.GenerateTitle(ctx, req.Content)
    }()
    awaitTitle = func() string { return <-ch }
}
result, err := h.council.RunFull(r.Context(), req.Content)
// ... if awaitTitle != nil: h.store.UpdateTitle(id, awaitTitle())
```

The goroutine uses `context.Background()` (not the request context) so it completes even if the client disconnects. A 30-second timeout bounds its lifetime.

### Error Handling

- Model query failures are logged; the caller skips failed results
- If all models fail in Stage 1, a descriptive error response is returned to the user
- If some models fail, the pipeline continues with successful responses
- `Stage3SynthesizeFinal` returns an error (not an embedded error string) if the chairman call fails
- Title generation failure is non-fatal; falls back to "New Conversation"
- Storage errors in the streaming path are logged (the SSE response has already started, so headers cannot be changed)
- Server handles `SIGINT`/`SIGTERM` with a 6-minute graceful shutdown window (allows in-flight council requests to complete)

## Logging

All log output uses `log/slog` with a JSON handler writing to stdout, configured at startup in `main.go`:

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
```

Every log entry is a structured JSON object. Error-level entries are emitted for failures that affect correctness (e.g., failed title update, failed assistant message persist). Warn-level entries are used for recoverable issues (e.g., corrupt storage files skipped during `List()`, `writeJSON` encoder errors). The `slog.Warn` in `writeJSON` is the only location that logs after the HTTP response has started writing.

## Dependencies

| Package | Purpose |
|---------|---------|
| `net/http` | HTTP server (stdlib) |
| `encoding/json` | JSON encode/decode (stdlib) |
| `log/slog` | Structured JSON logging (stdlib, Go 1.21+) |
| `sync` | WaitGroup + per-conversation Mutex (stdlib) |
| `math/rand` | Label shuffle for Stage 2 anonymization (stdlib) |
| `github.com/google/uuid` | Conversation ID generation |
| `github.com/joho/godotenv` | Load `.env` file |
| `honnef.co/go/tools/cmd/staticcheck` | Static analysis (tools-only build tag; invoked via `make lint`) |

Standard library covers HTTP, JSON, concurrency, and file I/O. External runtime dependencies are minimal.

## CORS

Allowed origins are configured via the `CORS_ORIGINS` environment variable (comma-separated). The default value allows `http://localhost:5173` (Vite) and `http://localhost:3000` for local development. The middleware checks each incoming `Origin` header against an allowlist built from `Config.CORSOrigins` at handler construction time.

## Running

```bash
make dev                     # go run ./cmd/server
make build && ./bin/llm-council  # compiled binary
make lint                    # go vet ./... && staticcheck ./...
make test                    # go test ./...
```

Frontend:
```bash
cd frontend && npm run dev
```
