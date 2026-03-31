# LLM Council — Backend

A Go HTTP backend implementing a **3-stage multi-LLM deliberation system**. Rather
than asking a single AI model for an answer, LLM Council assembles a council of
models that independently respond, anonymously review each other, and have a
designated Chairman synthesize a final answer.

The frontend (React + Vite) lives in the `frontend/` directory in this repo.

---

## Tech stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25+ |
| HTTP server | `net/http` (stdlib) |
| Concurrency | `sync.WaitGroup` + per-conversation `sync.Mutex` (stdlib) |
| Streaming | Server-Sent Events over `net/http` |
| Storage | JSON files on disk (no database) |
| LLM gateway | [OpenRouter](https://openrouter.ai) REST API |
| Config | Environment variables + `godotenv` |
| ID generation | `github.com/google/uuid` |
| Frontend | React + Vite (`frontend/` directory) |

---

## How it works

```
User query
    │
    ▼
┌─────────────────────────────────────┐
│ Stage 1 — Individual Responses      │
│ All council models answer in        │
│ parallel, unaware of each other     │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│ Stage 2 — Anonymized Peer Review    │
│ Each model ranks the others'        │
│ responses labeled A–Z               │
│ (labels are shuffled to avoid bias) │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│ Stage 3 — Chairman Synthesis        │
│ One designated model reads all      │
│ responses and rankings, then        │
│ writes the definitive final answer  │
└──────────────────┬──────────────────┘
                   │
                   ▼
              Final answer
```

**Why three stages?**

- **Stage 1** surfaces a diversity of perspectives — each model answers
  independently, so you get genuinely different approaches rather than one
  model's blind spots.
- **Stage 2** adds a credibility signal — peers evaluate each other without
  knowing authorship, producing a bias-reduced quality ranking ("street cred").
- **Stage 3** leverages the Chairman's synthesis ability — a single model reads
  all responses *and* the peer rankings, giving it the full picture to write
  the best possible answer.

---

## Prerequisites

- **Go 1.25+**
- An **[OpenRouter](https://openrouter.ai) API key** (provides access to all
  the LLMs through one endpoint)

---

## Quick start

```bash
# 1. Clone and enter the repo
git clone git@github.com:valpere/llm-council.git
cd llm-council

# 2. Create your .env file
cp .env.example .env
# Edit .env and set OPENROUTER_API_KEY

# 3. Run the development server
make dev
# → LLM Council API listening on :8001
```

Then start the frontend and open it in your browser:

```bash
cd frontend && npm ci && npm run dev
```

Or run both together with `make dev-all`.

---

## Configuration

All configuration is done via environment variables. Copy `.env.example` to
`.env` to get started.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENROUTER_API_KEY` | **Yes** | — | Your OpenRouter API key. The server starts without it but every LLM call will fail with 401. |
| `COUNCIL_MODELS` | No | 4 preset models¹ | Comma-separated list of OpenRouter model IDs to use as council members. |
| `CHAIRMAN_MODEL` | No | `google/gemini-3-pro-preview` | Model used for Stage 3 synthesis. |
| `DATA_DIR` | No | `data/conversations` | Directory where conversation JSON files are stored. |
| `PORT` | No | `8001` | TCP port the server listens on. |

¹ Default council: `openai/gpt-5.1`, `google/gemini-3-pro-preview`,
`anthropic/claude-sonnet-4.5`, `x-ai/grok-4`

---

## Development

```bash
make dev        # Run without compiling (go run ./cmd/server)
make build      # Compile to bin/llm-council
make run        # Compile then run the binary
make test       # Run all tests
make lint       # Run go vet ./...
make clean      # Remove bin/
```

> Always run `make` commands from the **project root**, not from a subdirectory.
> The server resolves `DATA_DIR` (default: `data/conversations/`) relative to the working directory.

---

## API reference

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Health check — returns `{"status":"ok","service":"LLM Council API"}` |
| `GET` | `/api/conversations` | List all conversations |
| `POST` | `/api/conversations` | Create a new conversation |
| `GET` | `/api/conversations/{id}` | Get a conversation with all messages |
| `POST` | `/api/conversations/{id}/message` | Send a message, wait for the full result |
| `POST` | `/api/conversations/{id}/message/stream` | Send a message, receive stage results via SSE |

### Streaming events (`/message/stream`)

The streaming endpoint uses
[Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events).
Each event is a `data:` line containing a JSON object with a `type` field:

```
data: {"type":"stage1_start"}
data: {"type":"stage1_complete","data":[...]}
data: {"type":"stage2_start"}
data: {"type":"stage2_complete","data":[...],"metadata":{"label_to_model":{...},"aggregate_rankings":[...]}}
data: {"type":"stage3_start"}
data: {"type":"stage3_complete","data":{...}}
data: {"type":"title_complete","data":{"title":"..."}}
data: {"type":"complete"}
```

On any error: `data: {"type":"error","message":"..."}` — the stream then closes.

### Conversation storage format

Each conversation is stored as a single JSON file under the directory configured
by `DATA_DIR` (default: `data/conversations/`):

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-01-01T12:00:00Z",
  "title": "My conversation",
  "messages": [
    { "role": "user", "content": "What is the best sorting algorithm?" },
    { "role": "assistant", "stage1": [...], "stage2": [...], "stage3": {...} }
  ]
}
```

---

## Project structure

```
llm-council/
├── cmd/server/main.go            Entry point — wires config → client → council → storage → api
├── internal/
│   ├── config/config.go          Config struct and Load() from environment variables
│   ├── openrouter/client.go      HTTP client for OpenRouter (parallel and single queries)
│   ├── council/
│   │   ├── council.go            3-stage pipeline: RunFull(), stage functions, ranking
│   │   └── types.go              Result types: StageOneResult, StageTwoResult, etc.
│   ├── storage/storage.go        JSON file persistence with atomic writes and per-conv locks
│   └── api/handler.go            HTTP handlers, CORS middleware, SSE streaming
├── docs/                         Architecture, stage logic, and implementation notes
├── data/conversations/           Created at runtime — one JSON file per conversation
├── Makefile
├── .env.example                  Template for all supported environment variables
└── go.mod                        Module: llm-council, Go 1.25+, two external deps
```

---

## Design notes

**Minimal dependencies.** The server uses only the Go standard library for HTTP,
JSON, concurrency, and file I/O. The two external packages are
`github.com/google/uuid` for conversation IDs and `github.com/joho/godotenv`
to load `.env` files. No framework, no ORM, no database.

**Atomic storage.** Conversation files are written to a `.tmp` file and then
renamed into place, so a crash mid-write never leaves a corrupt file. Concurrent
writes to the same conversation are serialized with a per-conversation mutex.

**Bias-free peer review.** Stage 2 labels (A–Z) are assigned to a *shuffled*
order of Stage 1 results each request, so no model is systematically favored by
always being "Response A". The label-to-model mapping is ephemeral — computed
per request, returned in the API response, but never persisted.

**Graceful degradation.** If one council model fails in Stage 1 or Stage 2, the
pipeline continues with the successful responses. On total Stage 1 failure the
behaviour depends on the endpoint: the JSON endpoint
(`POST /api/conversations/{id}/message`) returns HTTP 200 with an error message
embedded in `stage3.response`, while the streaming endpoint emits a
`{"type":"error",...}` SSE event and closes the stream.

---

## Frontend

The React UI lives in `frontend/`. It is a single-page app built with React 19 + Vite 8 (plain JavaScript, no TypeScript).

### Quick start

```bash
cd frontend && npm ci && npm run dev
```

The dev server starts on `:5173` and proxies all `/api` requests to the backend at `:8001`. Run `make dev-all` to start both together.

### Directory

```
frontend/
  src/
    api.js               # API adapter (all fetch calls)
    App.jsx              # root component + state
    components/          # Stage1, Stage2, Stage3, ChatInterface, Sidebar
  index.html
  vite.config.js
  package.json
```

See `docs/frontend/` for architecture, API contract, and SSE streaming docs.

---

## License

[MIT](LICENSE)
