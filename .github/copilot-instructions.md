# Copilot Instructions

## What this repo does

`llm-council-backend` is the Go HTTP backend for LLM Council — a 3-stage multi-LLM deliberation system:

1. **Stage 1** — council models answer the user query in parallel
2. **Stage 2** — each model anonymously peer-reviews and ranks the other responses (labels A/B/C/D, shuffled per request to prevent bias)
3. **Stage 3** — a designated Chairman model synthesizes a final answer

Conversations are persisted as JSON files. The React + Vite frontend lives in a sibling repository (`llm-council-frontend`) and connects via this API.

## Language and runtime

- **Go 1.25+**. The `go.mod` module name is `llm-council`.
- No CGo, no generated code.
- Runtime dependencies: `github.com/google/uuid` and `github.com/joho/godotenv`. Tool dependency: `honnef.co/go/tools/cmd/staticcheck` (pinned in `tools.go` via `//go:build tools`).

## Build, run, lint, test

```bash
make build       # go build -o bin/llm-council ./cmd/server
make dev         # go run ./cmd/server  (no compiled binary)
make run         # build then run bin/llm-council
make lint        # go vet ./... && go run honnef.co/go/tools/cmd/staticcheck ./...
make test        # go test ./...
make clean       # rm -rf bin/
```

Always run from the **project root** (not from a subdirectory). The binary resolves `data/conversations/` relative to the working directory.

**Environment:** create a `.env` file in the project root (see `.env.example`):

```
OPENROUTER_API_KEY=sk-or-v1-...
```

`godotenv.Load()` silently ignores a missing `.env`; the server will start but every OpenRouter call will fail with a 401. Optional overrides:

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_MODELS` | 4 preset models | Comma-separated list of OpenRouter model IDs |
| `CHAIRMAN_MODEL` | `google/gemini-3-pro-preview` | Model for Stage 3 synthesis |
| `DATA_DIR` | `data/conversations` | Directory for JSON conversation files |
| `PORT` | `8001` | TCP port the server listens on |

## Package layout

```
cmd/server/main.go            — entry point; wires config → openrouter → council → storage → api
internal/config/config.go     — Config struct, Load() reads env vars
internal/openrouter/client.go — QueryModel() / QueryModelsParallel() (sync.WaitGroup)
internal/council/types.go     — StageOneResult, StageTwoResult, StageThreeResult, Metadata, Result
internal/council/council.go   — Stage1…3, RunFull(), GenerateTitle(), CalculateAggregateRankings()
internal/storage/storage.go   — Create/Get/AddMessage/UpdateTitle/List; atomic writes; per-conv mutex
internal/api/handler.go       — HTTP handlers, CORS middleware, SSE streaming; all routes in Routes()
Makefile                      — build / dev / run / lint / test / clean targets
.env.example                  — template listing supported environment variables
.github/copilot-instructions.md — this file
.github/dependabot.yml        — weekly Go module and GitHub Actions dependency updates
docs/                         — architecture.md, council-stages.md, go-implementation.md
```

## Key design constraints

- **Storage IDs must be UUIDs** — `storage.Get/Create/AddMessage/UpdateTitle` validate against `^[0-9a-f]{8}-...$` and return an error for invalid IDs.
- **Atomic writes** — `storage.save()` writes to `{id}.json.tmp` then `os.Rename`; never write directly to `{id}.json`.
- **Per-conversation locking** — `storage.lockConv(id)` must wrap every read-modify-write cycle (`AddMessage`, `UpdateTitle`).
- **Stage 2 label limit** — `Stage2CollectRankings` returns an error if `len(stage1Results) > 26`.
- **Request body limit** — both `sendMessage` and `sendMessageStream` apply `http.MaxBytesReader(w, r.Body, 1<<20)` before decoding.
- **SSE format** — all streaming events are `data: {...}\n\n` with a `type` field in the JSON; no SSE `event:` line is used.
- **CORS** — only `http://localhost:5173` and `http://localhost:3000` are allowed origins (hardcoded in `corsMiddleware`); `Vary: Origin` is set when reflecting the origin.
- **File permissions** — data dir: `0700`; conversation files: `0600`.
- **Title generation** — `GenerateTitle` always uses `google/gemini-2.5-flash` (hardcoded).

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check → `{"status":"ok"}` |
| GET | `/api/conversations` | List conversations (metadata) |
| POST | `/api/conversations` | Create conversation → HTTP 200 |
| GET | `/api/conversations/{id}` | Get conversation with messages |
| POST | `/api/conversations/{id}/message` | Send message, full JSON response |
| POST | `/api/conversations/{id}/message/stream` | Send message, SSE stream |

`{id}` path values are validated as UUIDs by the storage layer.

## SSE event sequence (`/message/stream`)

```
data: {"type":"stage1_start"}
data: {"type":"stage1_complete","data":[...StageOneResult]}
data: {"type":"stage2_start"}
data: {"type":"stage2_complete","data":[...StageTwoResult],"metadata":{"label_to_model":{...},"aggregate_rankings":[...]}}
data: {"type":"stage3_start"}
data: {"type":"stage3_complete","data":{...StageThreeResult}}
data: {"type":"title_complete","data":{"title":"..."}}   ← only on first message
data: {"type":"complete"}
```

On any failure: `data: {"type":"error","message":"..."}` followed by return.

## Conversation JSON schema

```json
{
  "id": "<uuid>",
  "created_at": "<RFC3339>",
  "title": "New Conversation",
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "stage1": [...], "stage2": [...], "stage3": {...}}
  ]
}
```

`messages` is `[]json.RawMessage` — each element is either a user or assistant blob.

## Notes for the agent

- `math/rand` top-level functions are auto-seeded in Go 1.20+; no explicit seeding is needed.
- `os.Rename` is atomic on Linux (POSIX `rename(2)`); this project targets Linux only.
- The `sync.Map` in `Store.locks` grows with conversation count by design; one `*sync.Mutex` per UUID is acceptable.
- When adding tests, use real file I/O with `t.TempDir()` for storage tests — do not mock `os`.
- Run `make lint` (`go vet ./...`) before considering a change complete.
- The branch protection on `main` requires a pull request; never push directly to `main`.
