# Frontend Architecture

Single-page React application that presents the LLM Council 3-stage deliberation process as a chat interface.

## Tech Stack

| Tool | Version | Purpose |
|------|---------|---------|
| React | 19.2 | UI framework |
| Vite | 8.0 | Dev server + build tool |
| react-markdown | 10.1 | Render markdown in model responses |
| ESLint | 10 | Linting (flat config in `eslint.config.js`) |
| Node | ≥20.19 | Runtime requirement (`engines` in `package.json`) |

No Redux, no Context API, no TypeScript — state lives in `App.jsx` and flows down via props.

## File Structure

```
src/
├── api.js                     # API client (all backend calls, SSE adapter)
├── App.jsx                    # Root: state, streaming, message flow
├── App.css
├── main.jsx                   # React entry point
├── index.css
└── components/
    ├── Sidebar.jsx            # Conversation list + new conversation
    ├── Sidebar.css
    ├── ChatInterface.jsx      # Message thread, input form
    ├── ChatInterface.css
    ├── Stage1.jsx             # Tabbed view of individual model responses
    ├── Stage1.css
    ├── Stage2.jsx             # Peer rankings + aggregate scores table
    ├── Stage2.css
    ├── Stage3.jsx             # Final synthesized answer + error banner
    └── Stage3.css
```

## Component Tree

```
App
├── Sidebar
│   ├── "New Conversation" button
│   └── ConversationItem[] (title, message count)
└── ChatInterface
    ├── Message[] (user messages)
    └── AssistantMessage
        ├── Stage1  (tabs per model, markdown responses)
        ├── Stage2  (tabs per evaluator, aggregate ranking table)
        └── Stage3  (chairman's final answer, or error banner)
```

## Layered Architecture

```
App.jsx (state owner)
  ↓ props only
Components (Stage1, Stage2, Stage3, ChatInterface, Sidebar)
  ↑
src/api.js (SSE adapter — sole HTTP/SSE client)
```

**Immutable rules:**

1. **Components are pure UI.** They receive data via props and call handler functions passed from `App.jsx`. No direct calls to `src/api.js` or `fetch` from any component.

2. **`src/api.js` is the adapter boundary.** `onEvent(type, event)` is the only interface `App.jsx` sees. Raw SSE lines and HTTP status codes never leak past this boundary.

3. **`App.jsx` owns all state.** Only `App.jsx` writes to the assistant message shape via `setCurrentConversation`.

## State Management

All application state lives in `App.jsx`:

```javascript
conversations[]        // List metadata for sidebar
currentConversationId  // Active conversation ID
currentConversation    // Full object with messages[]
isLoading              // True while SSE stream is active
```

### Message Shape

**User message:**
```javascript
{ role: 'user', content: '...' }
```

**Assistant message (built progressively during streaming):**
```javascript
{
  role: 'assistant',
  stage1: null | [{model, response}, ...],
  stage2: null | [{model, ranking, parsed_ranking}, ...],
  stage3: null | {model, response},
  metadata: null | {label_to_model, aggregate_rankings},
  loading: { stage1: bool, stage2: bool, stage3: bool },
  error: null | string    // set on SSE error event; ephemeral, not persisted
}
```

The `loading` flags drive per-stage spinners. Fields start as `null` and are filled as SSE events arrive. Only `App.jsx` writes to this shape — components read it via props.

## Key Behaviours

### Optimistic Updates
When the user sends a message, two entries are immediately added to the message list — a user message and an empty assistant message — before any backend response arrives. This keeps the UI responsive.

### Progressive Streaming
`handleSendMessage` in `App.jsx` uses `api.sendMessageStream()` to open an SSE connection. Each event updates only the relevant part of the last assistant message via `setCurrentConversation`, causing React to re-render just the changed stage.

### SSE Chunk Buffering
`src/api.js` buffers incomplete lines across TCP chunks so that SSE events split at chunk boundaries are correctly reassembled before parsing. See `docs/streaming.md` for details.

### Auto-scroll
`ChatInterface.jsx` uses a `useRef` on the message container to scroll to the bottom whenever messages update.

### Markdown Rendering
All model responses (Stage 1, Stage 3) and ranking evaluations (Stage 2) are rendered through `react-markdown`. Using `dangerouslySetInnerHTML` is forbidden — it is an XSS risk with LLM-generated content.

### De-anonymization (Stage 2)
Stage 2 responses from the backend use generic labels (`Response A`, `Response B`, ...). `Stage2.jsx` replaces these with bold model names using the `label_to_model` map from `metadata.label_to_model`. This mapping is ephemeral — not stored by the backend — so it is only available during and immediately after the streaming response, not when loading a saved conversation.

### Error Handling (Stage 3)
If the SSE stream emits an `error` event, `App.jsx` sets `msg.error` and clears `loading.stage3`. `Stage3.jsx` renders an error banner when `msg.error` is set instead of a final answer.

## Configuration

`API_BASE` is read from the `VITE_API_BASE` environment variable at build time:

```javascript
const API_BASE = (() => {
  const raw = import.meta.env.VITE_API_BASE;
  if (typeof raw !== 'string') return 'http://localhost:8001';
  const trimmed = raw.trim().replace(/\/+$/, '');
  return trimmed || 'http://localhost:8001';
})();
```

Set `VITE_API_BASE` in a `.env` file (see `.env.example`). Defaults to `http://localhost:8001`.

## Dev Setup

```bash
npm install
npm run dev     # starts at http://localhost:5173
```

The Go backend must be running on port 8001 (or the port configured in `VITE_API_BASE`). CORS is configured on the backend for `localhost:5173`.
