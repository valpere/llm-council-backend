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

Single-page React application (served separately during development, or as embedded static files in production).

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
2. Backend saves the user message and triggers `RunFullCouncil()`
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
The `/message/stream` endpoint emits events as each stage completes (`stage1_complete`, `stage2_complete`, `stage3_complete`), enabling the frontend to display progressive updates rather than waiting for the full pipeline.

### Graceful Degradation
If a model query fails, the system continues with successful responses. Partial results are better than complete failure.

### Ephemeral Metadata
The `label_to_model` mapping and `aggregate_rankings` are computed per-request and not persisted to storage. They are returned in the API response and used only for display.

### JSON File Storage
Conversations are stored as individual JSON files in `data/conversations/{uuid}.json`. No database setup required. Simple and transparent.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/conversations` | List all conversations |
| `POST` | `/api/conversations` | Create a new conversation |
| `GET` | `/api/conversations/{id}` | Get conversation with messages |
| `POST` | `/api/conversations/{id}/message` | Send message, get full response |
| `POST` | `/api/conversations/{id}/message/stream` | Send message, stream stage events |

## External Services

- **OpenRouter** (`https://openrouter.ai/api/v1/chat/completions`): Unified gateway to multiple LLM providers. Requires `OPENROUTER_API_KEY`.
