# Go Implementation Plan

The original Python/FastAPI + React implementation is being rewritten in Go. The frontend remains React; only the backend changes.

## Package Structure

```
llm-council/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, server startup
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, Load() from env
│   ├── openrouter/
│   │   └── client.go        # QueryModel(), QueryModelsParallel()
│   ├── council/
│   │   ├── council.go       # RunFullCouncil(), stage functions
│   │   └── types.go         # StageOneResult, StageTwoResult, etc.
│   ├── storage/
│   │   ├── storage.go       # Load/Save/List conversations
│   │   └── types.go         # Conversation, Message structs
│   └── api/
│       ├── handler.go       # HTTP handlers
│       ├── routes.go        # Route registration
│       └── sse.go           # Server-Sent Events helpers
├── frontend/                # Unchanged React app
├── data/
│   └── conversations/       # JSON conversation files
├── go.mod
├── go.sum
└── .env
```

## Key Implementation Notes

### Concurrency

Use `errgroup` (or plain goroutines + channels) for parallel model queries:

```go
// Stage 1: query all models concurrently
results := make([]StageOneResult, len(models))
var wg sync.WaitGroup
for i, model := range models {
    wg.Add(1)
    go func(i int, model string) {
        defer wg.Done()
        resp, err := client.QueryModel(ctx, model, messages)
        if err == nil {
            results[i] = StageOneResult{Model: model, Response: resp}
        }
    }(i, model)
}
wg.Wait()
```

### SSE Streaming

Stage completion events are sent over a `text/event-stream` response. Each event carries JSON data:

```
event: stage1_complete
data: {"results": [...]}

event: stage2_complete
data: {"results": [...], "labelToModel": {...}, "aggregateRankings": [...]}

event: stage3_complete
data: {"result": {...}}

event: complete
data: {}
```

### Configuration

Loaded from environment variables (`.env` via `godotenv`):

```go
type Config struct {
    OpenRouterAPIKey string
    CouncilModels    []string
    ChairmanModel    string
    DataDir          string
    Port             int
}
```

### Storage

Each conversation is a single JSON file at `data/conversations/{uuid}.json`. The storage layer handles read/write with file locking if needed.

### Error Handling

- Model query failures return `("", err)` — the caller skips the result
- If all models fail in Stage 1, return an error to the user
- If some models fail, continue with successful responses
- Title generation failure is non-fatal; falls back to "New Conversation"

## Dependencies

| Package | Purpose |
|---------|---------|
| `net/http` | HTTP server (stdlib) |
| `encoding/json` | JSON encode/decode (stdlib) |
| `github.com/google/uuid` | Conversation ID generation |
| `github.com/joho/godotenv` | Load `.env` file |
| `golang.org/x/sync/errgroup` | Concurrent goroutine management |

Standard library covers HTTP, JSON, and file I/O. External dependencies are minimal.

## CORS

Allow `http://localhost:5173` (Vite dev server) and `http://localhost:3000` during development.

## Running

```bash
go run ./cmd/server          # Development
go build -o llm-council ./cmd/server && ./llm-council  # Production
```

Frontend remains:
```bash
cd frontend && npm run dev
```
