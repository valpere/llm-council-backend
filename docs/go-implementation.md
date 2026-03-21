# Go Implementation Notes

The original Python/FastAPI backend has been rewritten in Go. The frontend (React + Vite) is maintained separately in the `llm-council-frontend` repository.

## Package Structure

```
llm-council-backend/
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
│   │   ├── types.go         # StageOneResult, StageTwoResult, AggregateRanking, Result
│   │   ├── prompts.go       # rankingPromptTemplate, chairmanPromptTemplate, titlePromptTemplate
│   │   ├── council.go       # Council struct; stage functions; CalculateAggregateRankings
│   │   └── council_test.go  # Unit tests for parseRankingFromText, CalculateAggregateRankings
│   ├── storage/
│   │   └── storage.go       # Storer interface; Store struct; Create/Get/AddMessage/UpdateTitle/List
│   └── api/
│       └── handler.go       # HTTP handlers, CORS middleware, SSE streaming
├── data/
│   └── conversations/       # JSON conversation files
├── go.mod
├── go.sum
└── .env                     # Local secrets (not committed)
```

Frontend lives in the sibling repository `llm-council-frontend`.

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

data: {"type":"stage2_complete","data":[...],"metadata":{"label_to_model":{...},"aggregate_rankings":[...]}}

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
    Stage3SynthesizeFinal(ctx, query, stage1, stage2) (StageThreeResult, error)
    GenerateTitle(ctx, query) string
    RunFull(ctx, query) (Result, error)
    CalculateAggregateRankings(stage2, labelToModel) []AggregateRanking
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
}
```

### Storage

Each conversation is a single JSON file at `data/conversations/{uuid}.json`.

- Writes are atomic: data is written to a `.tmp` file then renamed, preventing partial writes on crash.
- Concurrent writes to the same conversation are serialized via a per-conversation `sync.Mutex`.
- Conversation IDs are validated against a UUID regex before any file path is constructed, preventing directory traversal.

### Title Generation

Title generation runs in a goroutine started before `RunFull()` so it overlaps with the council pipeline. It only runs for the first message in a conversation (`isFirst`):

```go
var titleCh chan string
if isFirst {
    titleCh = make(chan string, 1)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        titleCh <- h.council.GenerateTitle(ctx, req.Content)
    }()
}
result, err := h.council.RunFull(r.Context(), req.Content)
// ... if isFirst: h.store.UpdateTitle(id, <-titleCh)
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

## Dependencies

| Package | Purpose |
|---------|---------|
| `net/http` | HTTP server (stdlib) |
| `encoding/json` | JSON encode/decode (stdlib) |
| `sync` | WaitGroup + per-conversation Mutex (stdlib) |
| `math/rand` | Label shuffle for Stage 2 anonymization (stdlib) |
| `github.com/google/uuid` | Conversation ID generation |
| `github.com/joho/godotenv` | Load `.env` file |

Standard library covers HTTP, JSON, concurrency, and file I/O. External dependencies are minimal.

## CORS

Allowed origins are checked in `corsMiddleware`. Currently `http://localhost:5173` (Vite) and `http://localhost:3000` are accepted for local development. Moving origins to `Config` (from a `CORS_ORIGINS` env var) is tracked in issue #18.

## Running

```bash
make dev                     # go run ./cmd/server
make build && ./bin/llm-council  # compiled binary
```

Frontend (separate repo):
```bash
cd ../llm-council-frontend && npm run dev
```
