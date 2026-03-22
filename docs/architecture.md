# Architecture

LLM Council is a web application that implements a **3-stage deliberation system** using multiple LLMs collaboratively. Rather than querying a single model, users interact with a "council" of models that individually respond, peer-review each other, and have a designated "Chairman" synthesize a final answer.

## System Overview

```
User Query
    │
    ▼
┌─────────────────────────────────────┐
│ Stage 1: Individual Responses       │
│ All council models answer in        │
│ parallel                            │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│ Stage 2: Peer Review (Anonymized)   │
│ Models rank each other's responses  │
│ without knowing who wrote what      │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│ Stage 3: Chairman Synthesis         │
│ One model synthesizes all responses │
│ and rankings into a final answer    │
└──────────────────┬──────────────────┘
                   │
                   ▼
              Final Answer
```

## Components

### Backend (Go)

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Entry point, HTTP server setup |
| `internal/council` | Core 3-stage deliberation logic |
| `internal/openrouter` | OpenRouter API client |
| `internal/storage` | JSON-based conversation persistence |
| `internal/config` | Configuration loading |
| `internal/api` | HTTP handlers and routing |

### Frontend

Single-page React + Vite application in the separate `llm-council-frontend` repository (`../llm-council-frontend`).

| Component | Responsibility |
|-----------|---------------|
| `App.jsx` | Conversation state management, streaming |
| `ChatInterface.jsx` | Message thread display |
| `Stage1.jsx` | Tabbed view of individual model responses |
| `Stage2.jsx` | Peer rankings with de-anonymization |
| `Stage3.jsx` | Final synthesized answer |
| `Sidebar.jsx` | Conversation list navigation |

## Data Flow

1. User submits a query via the frontend
2. Backend saves the user message and triggers `RunFull()`
3. **Stage 1**: All council models queried concurrently via OpenRouter
4. **Stage 2**: Responses are anonymized (A/B/C/D labels), all models rank them concurrently; rankings are aggregated
5. **Stage 3**: Chairman model receives all responses + rankings and synthesizes a final answer
6. Results are saved to a JSON conversation file
7. Frontend receives updates via Server-Sent Events (SSE) as each stage completes

## Key Design Decisions

### Anonymized Peer Review
Models evaluate responses labeled "Response A/B/C/D" without knowing authorship. This prevents inter-model bias. The label-to-model mapping is created server-side and only revealed in the frontend for display purposes.

### Parallel Queries
Stage 1 and Stage 2 queries run concurrently using goroutines, reducing total latency from `N × model_latency` to `max(model_latency)`.

### Server-Sent Events (SSE)
The `/message/stream` endpoint emits progress as each stage completes. All events are sent as `data:` lines (standard SSE) containing a JSON object with a `type` field:

```
data: {"type":"stage1_complete","data":[...]}

data: {"type":"stage2_complete","data":[...],"metadata":{"label_to_model":{...},"aggregate_rankings":[...],"consensus_w":0.72}}

data: {"type":"stage3_complete","data":{...}}

data: {"type":"title_complete","data":{"title":"..."}}

data: {"type":"complete"}
```

This enables the frontend to display progressive updates rather than waiting for the full pipeline.

### Interface-Driven Dependency Injection

`council.Runner` and `storage.Storer` are interfaces defined near their consumers (`internal/council/interfaces.go` and `internal/storage/storage.go`). The HTTP handler depends only on these interfaces — never on concrete types. This makes handler tests possible without a real OpenRouter connection or real disk I/O.

```
Handler → council.Runner (interface) → *Council (concrete)
Handler → storage.Storer (interface) → *Store (concrete)
```

The same pattern applies one layer down: `Council` depends on `council.LLMClient`, not on `*openrouter.Client`, so the LLM layer can be mocked independently.

### Graceful Degradation
If a model query fails, the system continues with successful responses. Partial results are better than complete failure.

### Ephemeral Metadata
The `label_to_model` mapping and `aggregate_rankings` are computed per-request and not persisted to storage. They are returned in the API response and used only for display.

### Title Generation Parallelism
Title generation (a separate cheap LLM call) starts concurrently with `RunFull()`. It uses a detached `context.Background()` — not the request context — so it completes even if the client disconnects, and is bounded by a 30-second timeout.

### Kendall's W Consensus Score
After Stage 2, `CalculateAggregateRankings` computes Kendall's W (coefficient of concordance) across all council model rankings. W = 1.0 means perfect agreement; W = 0.0 means no agreement. The score is passed to the Chairman prompt with an English interpretation (strong / moderate / weak agreement) so the synthesis tone matches the actual degree of consensus. The `consensus_w` field is included in the `stage2_complete` SSE event and in the API response `Metadata`.

### Health Endpoints
Two dedicated health endpoints follow the standard liveness/readiness split. `/health/live` always returns 200 while the process is running. `/health/ready` performs a lightweight check (creates and stats the data directory) and returns 503 if the storage layer is unavailable. The root `/` endpoint is retained for backward compatibility.

### Structured Logging
The server uses `log/slog` with a JSON handler (written to stdout) from startup. All log entries include structured key-value context. Error-level entries are emitted for failures that affect correctness; warn-level for recoverable issues (e.g., skipped corrupt storage files, `writeJSON` encode failures).

### JSON File Storage
Conversations are stored as individual JSON files in `data/conversations/{uuid}.json`. No database setup required. Simple and transparent.

## API Endpoints

| Method | Path | Status | Description |
|--------|------|--------|-------------|
| `GET` | `/` | 200 | Legacy health check (returns `{"status":"ok","service":"LLM Council API"}`) |
| `GET` | `/health/live` | 200 | Liveness probe — process is running |
| `GET` | `/health/ready` | 200 / 503 | Readiness probe — data directory is accessible |
| `GET` | `/api/conversations` | 200 | List all conversations |
| `POST` | `/api/conversations` | 201 | Create a new conversation |
| `GET` | `/api/conversations/{id}` | 200 / 404 | Get conversation with messages |
| `POST` | `/api/conversations/{id}/message` | 200 | Send message, get full response |
| `POST` | `/api/conversations/{id}/message/stream` | 200 | Send message, stream stage events |

## External Services

- **OpenRouter** (`https://openrouter.ai/api/v1/chat/completions`): Unified gateway to multiple LLM providers. Requires `OPENROUTER_API_KEY`.
