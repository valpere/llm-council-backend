# LLM Council — User Guide

LLM Council is a backend API that runs a **3-stage multi-model deliberation pipeline**. Instead of asking one AI for an answer, it asks a council of models, has them anonymously peer-review each other, and uses a designated Chairman to synthesize a final answer from all inputs.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Configuration](#configuration)
3. [Running the Server](#running-the-server)
4. [API Reference](#api-reference)
5. [Using the Streaming Endpoint](#using-the-streaming-endpoint)
6. [Understanding the Response](#understanding-the-response)
7. [Health Checks](#health-checks)
8. [Data Storage](#data-storage)
9. [Integration Notes](#integration-notes)

---

## Quick Start

### Prerequisites

- Go 1.25 or later
- An [OpenRouter](https://openrouter.ai) API key with credits

### Setup

```bash
# Clone and enter the repo
cd llm-council

# Create .env with your API key
echo "OPENROUTER_API_KEY=sk-or-v1-..." > .env

# Run (must be from repo root)
go run ./cmd/server
```

The server starts on port **8001** by default. The frontend (in the `frontend/` directory) connects to this port.

---

## Configuration

All configuration is via environment variables. The server reads from `.env` at startup via the shell — there is no built-in `.env` loader; use `source .env` or a runner that handles it (the frontend dev proxy sets this up automatically).

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | *(required)* | Your OpenRouter API key. The server refuses to start if this is not set. |
| `COUNCIL_MODELS` | See below | Comma-separated list of model IDs to use as council members. |
| `CHAIRMAN_MODEL` | `google/gemini-3-pro-preview` | Model that synthesizes the final answer in Stage 3. |
| `TITLE_MODEL` | `google/gemini-2.5-flash` | Model used to auto-generate conversation titles. |
| `PORT` | `8001` | TCP port the HTTP server listens on. |
| `DATA_DIR` | `data/conversations` | Directory where conversation JSON files are stored. Relative to the working directory. |
| `CORS_ORIGINS` | `http://localhost:5173,http://localhost:3000` | Comma-separated list of allowed CORS origins. Set this to your frontend URL in production. |

### Default council models

```
openai/gpt-5.1
google/gemini-3-pro-preview
anthropic/claude-sonnet-4.5
x-ai/grok-4
```

### Custom council example

```bash
COUNCIL_MODELS="openai/gpt-4o,anthropic/claude-3-5-sonnet,google/gemini-flash-1.5" \
CHAIRMAN_MODEL="openai/gpt-4o" \
go run ./cmd/server
```

Any model available on OpenRouter can be used. The council works best with at least 3 models.

---

## Running the Server

```bash
# Development (from repo root)
go run ./cmd/server

# Build and run
go build -o llm-council ./cmd/server
./llm-council

# With custom config
OPENROUTER_API_KEY=sk-or-v1-... PORT=9000 go run ./cmd/server
```

**Important:** Always run from the repo root, not from `cmd/server/`. The default data directory (`data/conversations`) is relative to the working directory.

On startup the server logs its configuration to stdout as structured JSON:

```json
{"time":"...","level":"INFO","msg":"server starting","port":"8001"}
```

---

## API Reference

Base URL: `http://localhost:8001`

### Conversations

#### `GET /api/conversations`

Returns all conversations, sorted by creation time (newest first).

**Response** `200 OK`:
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "created_at": "2026-03-22T09:00:00Z",
    "title": "What is the Fermi paradox?",
    "message_count": 2
  }
]
```

Returns `[]` when no conversations exist.

---

#### `POST /api/conversations`

Creates a new empty conversation.

**Response** `201 Created`:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-22T09:00:00Z",
  "title": "New Conversation",
  "messages": []
}
```

---

#### `GET /api/conversations/{id}`

Returns a full conversation including all messages.

**Response** `200 OK`:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-22T09:00:00Z",
  "title": "What is the Fermi paradox?",
  "messages": [...]
}
```

Returns `404` if the conversation does not exist.

---

#### `POST /api/conversations/{id}/message`

Sends a message and waits for the full 3-stage pipeline to complete before responding. Use this for simple integrations that do not need streaming.

**Request body**:
```json
{ "content": "What is the Fermi paradox?" }
```

**Response** `200 OK` — see [Understanding the Response](#understanding-the-response).

---

#### `POST /api/conversations/{id}/message/stream`

Sends a message and streams stage events as Server-Sent Events (SSE). Use this for UIs that want to show progressive updates.

**Request body**: same as `/message`

**Response**: `text/event-stream` — see [Using the Streaming Endpoint](#using-the-streaming-endpoint).

---

### Error responses

All error responses use the same shape:

```json
{ "error": "description of what went wrong" }
```

| Status | When |
|--------|------|
| `400` | Malformed JSON body or request too large (> 1 MB) |
| `404` | Conversation not found |
| `500` | Internal error (storage failure, all models failed) |

---

## Using the Streaming Endpoint

The streaming endpoint emits [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events). Each event is a `data:` line containing a JSON object with a `type` field.

### Event sequence

```
→ POST /api/conversations/{id}/message/stream

← data: {"type":"stage1_start"}
← data: {"type":"stage1_complete","data":[...]}

← data: {"type":"stage2_start"}
← data: {"type":"stage2_complete","data":[...],"metadata":{...}}

← data: {"type":"stage3_start"}
← data: {"type":"stage3_complete","data":{...}}

← data: {"type":"title_complete","data":{"title":"..."}}   ← first message only
← data: {"type":"complete"}
```

### Event payloads

#### `stage1_complete`

```json
{
  "type": "stage1_complete",
  "data": [
    { "model": "openai/gpt-5.1",        "response": "The Fermi paradox is..." },
    { "model": "google/gemini-3-pro-preview", "response": "Named after Enrico Fermi..." },
    { "model": "anthropic/claude-sonnet-4.5", "response": "..." },
    { "model": "x-ai/grok-4",           "response": "..." }
  ]
}
```

#### `stage2_complete`

```json
{
  "type": "stage2_complete",
  "data": [
    { "model": "openai/gpt-5.1", "ranking": "1. Response C\n2. Response A\n...", "parsed_ranking": ["C","A","B","D"] }
  ],
  "metadata": {
    "label_to_model": { "A": "openai/gpt-5.1", "B": "google/gemini-3-pro-preview", "C": "anthropic/claude-sonnet-4.5", "D": "x-ai/grok-4" },
    "aggregate_rankings": [
      { "model": "anthropic/claude-sonnet-4.5", "average_rank": 1.75, "rankings_count": 4 },
      { "model": "openai/gpt-5.1",              "average_rank": 2.25, "rankings_count": 4 }
    ],
    "consensus_w": 0.72
  }
}
```

`aggregate_rankings` is sorted ascending by `average_rank` (rank 1 = best). `consensus_w` is Kendall's W coefficient (0–1): ≥ 0.7 indicates strong agreement among reviewers on which responses are best.

#### `stage3_complete`

```json
{
  "type": "stage3_complete",
  "data": {
    "model": "google/gemini-3-pro-preview",
    "response": "The Fermi paradox, named after physicist Enrico Fermi, asks..."
  }
}
```

#### `title_complete` *(first message in a conversation only)*

```json
{ "type": "title_complete", "data": { "title": "Fermi Paradox Explained" } }
```

#### `error`

```json
{ "type": "error", "message": "All models failed to respond. Please try again." }
```

An `error` event means the pipeline failed mid-stream. The connection remains open until `complete` or an error terminates it.

### Consuming SSE in JavaScript

```js
const es = new EventSource(''); // not applicable for POST — use fetch

const response = await fetch(`/api/conversations/${id}/message/stream`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ content: 'What is the Fermi paradox?' }),
});

const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  buffer += decoder.decode(value, { stream: true });
  const lines = buffer.split('\n');
  buffer = lines.pop(); // keep incomplete line
  for (const line of lines) {
    if (!line.startsWith('data: ')) continue;
    const event = JSON.parse(line.slice(6));
    switch (event.type) {
      case 'stage1_complete': console.log('Stage 1:', event.data); break;
      case 'stage3_complete': console.log('Final:', event.data.response); break;
      case 'complete':        console.log('Done'); break;
    }
  }
}
```

---

## Understanding the Response

Both `/message` and `/message/stream` (after `complete`) represent the same data shape:

```json
{
  "stage1": [
    { "model": "openai/gpt-5.1", "response": "..." },
    ...
  ],
  "stage2": [
    { "model": "openai/gpt-5.1", "ranking": "1. Response C...", "parsed_ranking": ["C","A","B","D"] },
    ...
  ],
  "stage3": {
    "model": "google/gemini-3-pro-preview",
    "response": "..."
  },
  "metadata": {
    "label_to_model": { "A": "openai/gpt-5.1", ... },
    "aggregate_rankings": [
      { "model": "...", "average_rank": 1.75, "rankings_count": 4 }
    ],
    "consensus_w": 0.72
  }
}
```

### Consensus W (Kendall's W)

`metadata.consensus_w` measures inter-reviewer agreement on the rankings (0 = no agreement, 1 = perfect agreement):

| Value | Interpretation |
|-------|---------------|
| ≥ 0.7 | **Strong consensus** — reviewers agree clearly on quality order |
| 0.4–0.7 | **Moderate consensus** — partial agreement |
| < 0.4 | **Weak consensus** — reviewers disagree significantly |

The Chairman model receives the consensus level as part of its synthesis prompt and adjusts its tone accordingly — a strong consensus lets the Chairman speak with more confidence about which response was best.

### Aggregate rankings

`metadata.aggregate_rankings` lists models sorted by average rank across all reviewers (lower = better). Use this to see which model the council collectively preferred for this query.

---

## Health Checks

| Endpoint | Status | When |
|----------|--------|------|
| `GET /` | `200` | Always — legacy compatibility check |
| `GET /health/live` | `200` | Always — process is alive |
| `GET /health/ready` | `200` or `503` | 200 when data directory is accessible; 503 otherwise |

Use `/health/live` for liveness probes and `/health/ready` for readiness probes in container orchestration.

```bash
curl http://localhost:8001/health/ready
# {"status":"ok"}
```

A `503` from `/health/ready` means the data directory is unavailable. Check `DATA_DIR` permissions.

---

## Data Storage

Conversations are stored as individual JSON files:

```
data/conversations/
  550e8400-e29b-41d4-a716-446655440000.json
  7c9e6679-7425-40de-944b-e07fc1f90ae7.json
  ...
```

Each file is a self-contained JSON object with the full conversation including all messages and stage results. Files are written atomically (write to `.tmp`, then rename) to prevent corruption on crash.

### Backup

Simply copy the `data/conversations/` directory. Each file is independent.

### Changing the data directory

```bash
DATA_DIR=/var/lib/llm-council/conversations go run ./cmd/server
```

The directory is created automatically on first use with permissions `0700`.

---

## Integration Notes

### CORS

The server allows cross-origin requests from origins listed in `CORS_ORIGINS`. For production, set this explicitly:

```bash
CORS_ORIGINS="https://your-frontend.example.com" go run ./cmd/server
```

Preflight `OPTIONS` requests are handled automatically.

### Request size limit

Request bodies are limited to **1 MB**. Requests exceeding this return `400 Bad Request`.

### Model timeouts

Each individual model query has a **120-second timeout**. If a model does not respond within 120 seconds, it is skipped and the pipeline continues with successful responses. The overall request does not fail unless all models time out.

### Title generation

Conversation titles are generated asynchronously after the first message using `TITLE_MODEL`. Title generation runs in the background with a 30-second timeout and does not block the response. If title generation fails, the conversation retains the title `"New Conversation"`.

### Structured logs

The server logs to stdout as structured JSON using `log/slog`. Log level is `INFO` by default. Errors during request handling are logged at `ERROR`; minor issues (like a client disconnecting during a write) at `WARN`.

```json
{"time":"2026-03-22T09:00:00Z","level":"INFO","msg":"server starting","port":"8001"}
{"time":"2026-03-22T09:00:05Z","level":"WARN","msg":"writeJSON: encode failed","status":200,"error":"write tcp: broken pipe"}
```

## Frontend Usage

See [`docs/frontend/user-guide.md`](frontend/user-guide.md) for the end-user UI walkthrough.
