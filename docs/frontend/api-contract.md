# API Contract

The frontend communicates with the Go backend (port 8001 by default) via REST + Server-Sent Events. REST endpoints send and receive JSON bodies, while the streaming endpoint uses `text/event-stream` (SSE) and emits events whose `data:` fields contain JSON payloads.

## Design Constraints

- **One question per conversation.** Each conversation stores exactly one user message and one assistant message. Sending a second message to an existing conversation is not supported by the UI — the frontend creates a new conversation for each question.
- **`metadata` is ephemeral (v1).** The `label_to_model` and `aggregate_rankings` fields are only returned during the streaming response and are not persisted. `GET /api/conversations/{id}` does not include them. **v2 note:** the resolved design decision for v2 is to persist full metadata with every assistant message — see `docs/council-research-gaps.md`.

---

## Endpoints

### List Conversations

```
GET /api/conversations
```

Response:
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "created_at": "2025-03-21T10:00:00.000000Z",
    "title": "Why is the sky blue?",
    "message_count": 2
  }
]
```

Sorted by `created_at` descending (newest first). Used by `Sidebar` to populate the conversation list.

---

### Create Conversation

```
POST /api/conversations
Content-Type: application/json

{}
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-03-21T10:00:00.000000Z",
  "title": "New Conversation",
  "messages": []
}
```

---

### Get Conversation

```
GET /api/conversations/{id}
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-03-21T10:00:00.000000Z",
  "title": "Why is the sky blue?",
  "messages": [
    {
      "role": "user",
      "content": "Why is the sky blue?"
    },
    {
      "role": "assistant",
      "stage1": [
        { "model": "openai/gpt-5.1", "response": "..." }
      ],
      "stage2": [
        {
          "model": "openai/gpt-5.1",
          "ranking": "Full evaluation text...",
          "parsed_ranking": ["Response C", "Response A", "Response B"]
        }
      ],
      "stage3": {
        "model": "google/gemini-3-pro-preview",
        "response": "Final synthesized answer..."
      }
    }
  ]
}
```

Note: `metadata` (`label_to_model`, `aggregate_rankings`) is **not persisted** — it is only returned during the streaming response. `GET` responses will not include it.

---

### Send Message (Blocking)

```
POST /api/conversations/{id}/message
Content-Type: application/json

{"content": "Why is the sky blue?"}
```

Response: same assistant message shape as stored in `GET /api/conversations/{id}`, plus a top-level `metadata` field:

```json
{
  "stage1": [...],
  "stage2": [...],
  "stage3": {...},
  "metadata": {
    "label_to_model": {
      "Response A": "openai/gpt-5.1",
      "Response B": "anthropic/claude-sonnet-4.5",
      "Response C": "google/gemini-3-pro-preview",
      "Response D": "x-ai/grok-4"
    },
    "aggregate_rankings": [
      { "model": "openai/gpt-5.1", "average_rank": 1.5, "rankings_count": 4 }
    ]
  }
}
```

The frontend uses the streaming endpoint instead. This endpoint exists for non-streaming use cases.

---

### Send Message (Streaming)

```
POST /api/conversations/{id}/message/stream
Content-Type: application/json

{"content": "Why is the sky blue?"}
```

Response: `text/event-stream` (Server-Sent Events). See [streaming.md](./streaming.md) for the full event sequence and shapes.

---

## Data Types

> These are JSON object shapes. Property types use pseudocode notation (`string`, `number`, `bool`, `array[]`).

### ConversationMeta

```
{
  id: string           // UUID v4
  created_at: string   // RFC 3339 / ISO 8601
  title: string
  message_count: number
}
```

### Conversation

```
{
  id: string
  created_at: string
  title: string
  messages: (UserMessage | AssistantMessage)[]
}
```

### UserMessage

```
{
  role: "user"
  content: string
}
```

### AssistantMessage (stored)

```
{
  role: "assistant"
  stage1: StageOneResult[]
  stage2: StageTwoResult[]
  stage3: StageThreeResult
}
```

### StageOneResult

```
{ model: string; response: string }
```

### StageTwoResult

```
{
  model: string
  ranking: string           // full evaluation text from the model
  parsed_ranking: string[]  // extracted ordered list: ["Response C", "Response A", ...]
}
```

### StageThreeResult

```
{ model: string; response: string }
```

### Metadata (ephemeral — streaming response only, not stored)

```
{
  label_to_model: { [label: string]: string }  // "Response A" → "openai/gpt-5.1"
  aggregate_rankings: {
    model: string
    average_rank: number      // lower is better (1.0 = always ranked first)
    rankings_count: number
  }[]
}
```

---

## CORS

The backend allows:
- Origins: `http://localhost:5173`, `http://localhost:3000`
- Methods: `GET`, `POST`, `OPTIONS`
- Headers: `Content-Type`
