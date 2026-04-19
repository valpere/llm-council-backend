# LLM Council — HTTP API Reference

## Base URL

The server listens on port `PORT` (default `8001`) and binds on all interfaces (`0.0.0.0`)
by default. For local development: `http://localhost:{PORT}`. From other machines:
`http://<host>:{PORT}`.

The frontend dev server (Vite at `:5173`) proxies `/api` requests to the backend, so no
additional CORS headers are needed during development when using that proxy.

---

## CORS

Allowed origins (hardcoded):

- `http://localhost:5173`
- `http://localhost:3000`

When the request `Origin` header matches, the server reflects it and sets:

```
Access-Control-Allow-Origin: <origin>
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type
Vary: Origin
```

Preflight `OPTIONS` requests return `204 No Content`.

---

## Request limits

- **Body size:** 1 MiB (`http.MaxBytesReader`). Requests exceeding this limit receive `400 Bad Request`.

---

## Error format

All error responses share one shape:

```json
{ "error": "human-readable message" }
```

Common status codes:

| Code | Meaning |
|------|---------|
| 400 | Invalid request body or malformed UUID path parameter |
| 404 | Conversation not found |
| 503 | Council quorum not met (too many models failed) |
| 500 | Internal server error |

---

## Routes

### `GET /health/live`

Liveness probe.

**Response `200 OK`** — empty body.

---

### `GET /health/ready`

Readiness probe.

**Response `200 OK`** — empty body.

---

### `GET /api/conversations`

List all conversations, newest first.

**Response `200 OK`**

```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "created_at": "2024-01-15T10:30:00Z",
    "title": "Explain the trolley problem",
    "message_count": 4
  }
]
```

Returns `[]` (empty array) when no conversations exist.

---

### `POST /api/conversations`

Create a new conversation.

**Request** — no body required.

**Response `201 Created`**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2024-01-15T10:30:00Z",
  "title": "New Conversation",
  "messages": []
}
```

---

### `GET /api/conversations/{id}`

Fetch a conversation with its full message history.

**Path parameter** — `id`: UUID v4.

**Response `200 OK`**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2024-01-15T10:30:00Z",
  "title": "Explain the trolley problem",
  "messages": [
    { "role": "user", "content": "Explain the trolley problem" },
    {
      "role": "assistant",
      "stage1": [ /* StageOneResult[] */ ],
      "stage2": [ /* StageTwoResult[] */ ],
      "stage3": { /* StageThreeResult */ },
      "metadata": { /* Metadata */ }
    }
  ]
}
```

`messages` is a heterogeneous array. Demux by the `"role"` field:
- `"user"` — `{ role, content }`
- `"assistant"` — `{ role, stage1, stage2, stage3, metadata }`

**Errors:** `400` (invalid UUID), `404` (not found).

---

### `POST /api/conversations/{id}/message`

Send a message and receive the full deliberation result in a single JSON response
(blocking — waits for all three stages to complete).

**Path parameter** — `id`: UUID v4.

**Request body**

```json
{
  "content": "Explain the trolley problem",
  "council_type": "default"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | The user's message |
| `council_type` | string | no | Council strategy name; defaults to `DEFAULT_COUNCIL_TYPE` env var |

**Response `200 OK`** — `AssistantMessage`

```json
{
  "role": "assistant",
  "stage1": [
    {
      "label": "Response A",
      "content": "...",
      "model": "openai/gpt-4o-mini",
      "duration_ms": 1240
    }
  ],
  "stage2": [
    {
      "reviewer_label": "Response B",
      "rankings": ["Response A", "Response C", "Response B"]
    }
  ],
  "stage3": {
    "content": "...",
    "model": "openai/gpt-4o-mini",
    "duration_ms": 890
  },
  "metadata": {
    "council_type": "default",
    "label_to_model": { "Response A": "openai/gpt-4o-mini", "Response B": "..." },
    "aggregate_rankings": [
      { "model": "openai/gpt-4o-mini", "score": 1.5 }
    ],
    "consensus_w": 0.83
  }
}
```

**Errors:** `400` (invalid body/UUID), `404` (not found), `503` (quorum not met), `500`.

---

### `POST /api/conversations/{id}/message/stream`

Send a message and receive the deliberation result as a Server-Sent Events stream.
Each event is flushed immediately as the stage completes — no polling required.

**Path parameter** — `id`: UUID v4.

**Request body** — same as `/message`.

**Response headers**

```
Content-Type: text/event-stream
Cache-Control: no-cache
X-Accel-Buffering: no
```

**SSE event format**

Every event is a single `data:` line followed by a blank line:

```
data: {"type":"<event_type>",...}\n\n
```

There is no `event:` line — demux by the `"type"` field of the JSON payload.

---

## SSE event sequence

```
data: {"type":"stage1_complete","data":[...StageOneResult]}
data: {"type":"stage2_complete","data":[...StageTwoResult],"metadata":{...Metadata}}
data: {"type":"stage3_complete","data":{...StageThreeResult}}
data: {"type":"title_complete","data":{"title":"..."}}     ← may be absent if title generation times out
data: {"type":"complete"}
```

On failure at any point:

```
data: {"type":"error","message":"human-readable message"}
```

After an error event the stream ends. No `complete` event follows.

### `stage1_complete`

Emitted when all council models have responded in Stage 1.

```json
{
  "type": "stage1_complete",
  "data": [
    {
      "label": "Response A",
      "content": "...",
      "model": "openai/gpt-4o-mini",
      "duration_ms": 1240
    },
    {
      "label": "Response B",
      "content": "...",
      "model": "anthropic/claude-haiku-4-5",
      "duration_ms": 980
    }
  ]
}
```

Labels are assigned sequentially (`A`, `B`, `C`, …). The mapping of label → model is
revealed in `metadata.label_to_model` at `stage2_complete`.

### `stage2_complete`

Emitted when all peer-review rankings are computed. `metadata` is a **top-level field**
on the event object, not nested inside `data`.

```json
{
  "type": "stage2_complete",
  "data": [
    {
      "reviewer_label": "Response B",
      "rankings": ["Response A", "Response C", "Response B"]
    }
  ],
  "metadata": {
    "council_type": "default",
    "label_to_model": {
      "Response A": "openai/gpt-4o-mini",
      "Response B": "anthropic/claude-haiku-4-5"
    },
    "aggregate_rankings": [
      { "model": "openai/gpt-4o-mini", "score": 1.5 },
      { "model": "anthropic/claude-haiku-4-5", "score": 2.5 }
    ],
    "consensus_w": 0.83
  }
}
```

`aggregate_rankings` are sorted by `score` ascending (lower = better rank).
`consensus_w` is a 0–1 weight indicating agreement across reviewers.

### `stage3_complete`

Emitted when the Chairman model has synthesised the final answer.

```json
{
  "type": "stage3_complete",
  "data": {
    "content": "The trolley problem is a thought experiment...",
    "model": "openai/gpt-4o-mini",
    "duration_ms": 1100
  }
}
```

### `title_complete`

Emitted after `stage3_complete` when title generation succeeds. May be **absent** if
title generation times out (30-second deadline). The title is derived from the first
50 **bytes** of the Stage 3 response — responses containing multi-byte UTF-8 characters
may be cut mid-character.

```json
{
  "type": "title_complete",
  "data": { "title": "The trolley problem is a thought experimen" }
}
```

### `complete`

Signals the end of the stream. No payload.

```json
{ "type": "complete" }
```

### `error`

Emitted when the pipeline fails. Stream ends after this event.

```json
{ "type": "error", "message": "council quorum not met" }
```

---

## Type reference

### `StageOneResult`

| Field | Type | Description |
|-------|------|-------------|
| `label` | string | Anonymised label, e.g. `"Response A"` |
| `content` | string | Model's answer |
| `model` | string | OpenRouter model ID |
| `duration_ms` | number | Wall-clock time for this model's response |

### `StageTwoResult`

| Field | Type | Description |
|-------|------|-------------|
| `reviewer_label` | string | Label of the reviewing model |
| `rankings` | string[] | Labels ordered best-first |

### `StageThreeResult`

| Field | Type | Description |
|-------|------|-------------|
| `content` | string | Chairman's synthesised answer |
| `model` | string | OpenRouter model ID |
| `duration_ms` | number | Wall-clock time |

### `Metadata`

| Field | Type | Description |
|-------|------|-------------|
| `council_type` | string | Strategy name used for this run |
| `label_to_model` | object | Maps label → OpenRouter model ID |
| `aggregate_rankings` | `RankedModel[]` | Models sorted by aggregate score (ascending) |
| `consensus_w` | number | Consensus weight 0–1 |

### `RankedModel`

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | OpenRouter model ID |
| `score` | number | Aggregate rank score (lower = ranked higher overall) |
