# LLM Council — Copilot Instructions

## What this repo does

`llm-council` is a multi-LLM deliberation system with a Go HTTP backend and a React + Vite frontend.

**Pipeline (3 stages, streamed over SSE):**
1. **Stage 1** — council models answer the user query in parallel
2. **Stage 2** — each model anonymously peer-reviews and ranks the other responses (labels A/B/C/D, shuffled per request)
3. **Stage 3** — a Chairman model synthesises the final answer using aggregate rankings

Conversations are persisted as JSON files on disk.

## Language and runtime

- **Go 1.26+**. Module name: `llm-council`.
- No CGo, no generated code.
- Runtime dependencies: `github.com/joho/godotenv` only. UUIDs use `crypto/rand` (no uuid package).

## Build, run, lint, test

```bash
make build       # go build -o bin/llm-council ./cmd/server
make dev         # go run ./cmd/server
make lint        # go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@v0.5.1 ./...
make test        # go test -race -count=1 ./...
make clean       # rm -f bin/llm-council

make fr-dev      # cd frontend && npm run dev  (Vite at :5173)
make fr-build    # cd frontend && npm run build
make fr-lint     # cd frontend && npm run lint
```

Always run from the **project root**. The binary resolves `data/conversations/` relative to cwd.

**Environment:** copy `.env.example` to `.env`:

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | — | Required. API key for OpenRouter (or compatible provider). |
| `COUNCIL_MODELS` | 4 preset models | Comma-separated OpenRouter model IDs |
| `CHAIRMAN_MODEL` | `google/gemini-3.1-pro-preview` | Model for Stage 3 synthesis |
| `DEFAULT_COUNCIL_TYPE` | `default` | Council strategy |
| `DEFAULT_COUNCIL_TEMPERATURE` | `0.7` | LLM temperature |
| `DATA_DIR` | `data/conversations` | Directory for JSON conversation files |
| `PORT` | `8001` | TCP port |
| `LLM_API_BASE_URL` | `https://openrouter.ai/api/v1` | Override for Ollama or any OpenAI-compatible endpoint |

## Package layout

```
cmd/server/main.go            — entry point; wires config → openrouter → council → storage → api
internal/config/config.go     — Config struct, Load() reads and validates env vars
internal/openrouter/client.go — QueryModel() / QueryModelsParallel() (sync.WaitGroup)
internal/council/types.go     — StageOneResult, StageTwoResult, StageThreeResult, Metadata, Result
internal/council/council.go   — Stage1…3, RunFull(), GenerateTitle(), CalculateAggregateRankings()
internal/storage/storage.go   — Create/Get/AddMessage/UpdateTitle/List; atomic writes; per-conv mutex
internal/api/handler.go       — HTTP handlers, CORS middleware, SSE streaming; all routes in Routes()
```

## Layer boundaries (strict — never violate)

```
cmd/server/main.go      — wiring only; no business logic
internal/api/           — parse request → call interfaces → write response; no logic
internal/council/       — deliberation; no net/http, no storage
internal/storage/       — persistence; no net/http, no council
internal/openrouter/    — LLM API client; no council, no storage
internal/config/        — env loading and validation only
```

Cross-layer calls go through interfaces at the consumer boundary. `internal/api` must not import `internal/storage` or `internal/openrouter` directly — it uses interfaces.

## Key design constraints

- **Atomic writes** — `storage.save()` writes to `{id}.json.tmp` then `os.Rename`; never write to `{id}.json` directly.
- **Per-conversation locking** — `storage.lockConv(id)` wraps every read-modify-write cycle.
- **Stage 2 label limit** — returns an error if `len(stage1Results) > 26`.
- **Request body limit** — `http.MaxBytesReader(w, r.Body, 1<<20)` before decoding.
- **SSE format** — all streaming events are `data: {...}\n\n` with a `type` field; no SSE `event:` line.
- **CORS** — allowed origins in config (no hardcoded values); `Vary: Origin` set when reflecting.
- **File permissions** — data dir: `0700`; conversation files: `0600`.

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check → `{"status":"ok"}` |
| GET | `/api/conversations` | List conversations (metadata) |
| POST | `/api/conversations` | Create conversation → HTTP 200 |
| GET | `/api/conversations/{id}` | Get conversation with messages |
| POST | `/api/conversations/{id}/message` | Send message, full JSON response |
| POST | `/api/conversations/{id}/message/stream` | Send message, SSE stream |

## SSE event sequence

```
data: {"type":"stage1_start"}
data: {"type":"stage1_complete","data":[...StageOneResult]}
data: {"type":"stage2_start"}
data: {"type":"stage2_complete","data":[...StageTwoResult],"metadata":{...}}
data: {"type":"stage3_start"}
data: {"type":"stage3_complete","data":{...StageThreeResult}}
data: {"type":"title_complete","data":{"title":"..."}}   ← first message only
data: {"type":"complete"}
```

On failure: `data: {"type":"error","message":"..."}` then return.

## Frontend

**Stack:** React 19 + Vite 8, plain JavaScript (no TypeScript), ESM modules.
**Directory:** `frontend/`

**Architecture rules (immutable — flag any violation in review):**
1. Components are pure UI — no `fetch` calls, no imports from `api.js`.
2. `src/api.js` is the sole network boundary. `onEvent(type, event)` is the only SSE interface `App.jsx` sees.
3. `App.jsx` owns all state — only `App.jsx` calls `setCurrentConversation` / `setConversations`.
4. `react-markdown` is the only renderer for LLM output — `dangerouslySetInnerHTML` is forbidden (XSS risk).

**Source layout:**
```
frontend/src/
  api.js              — sole fetch adapter; defaults to relative URLs (Vite proxy in dev)
  App.jsx             — root component; owns all application state
  utils.js            — shared utilities (e.g. stripMarkdown)
  theme.css           — design tokens (CSS custom properties)
  components/
    ChatInterface.jsx — message thread + always-visible input form
    Sidebar.jsx       — conversation list, theme toggle, collapse
    Stage1.jsx        — tabbed individual model responses (accordion, collapsed by default)
    Stage2.jsx        — peer rankings + consensus badge (accordion, collapsed by default)
    Stage3.jsx        — chairman synthesis hero card (always expanded)
    EmptyState.jsx    — welcome screen with prompt chips
    Markdown.jsx      — shared react-markdown wrapper with syntax highlighting
    *.css             — co-located CSS per component
```

**CSS conventions:** use `var(--token)` from `theme.css` — no hardcoded colour values.

**Dev proxy:** `vite.config.js` reads `PORT` from the root `.env` and proxies `/api` to `http://localhost:{PORT}`. `VITE_API_BASE` is only for cross-origin production deployments.

**No test suite.** `npm run lint` is the quality gate.

## Notes for reviewers

- `math/rand` top-level functions are auto-seeded in Go 1.20+; no explicit seeding needed.
- `os.Rename` is atomic on Linux (POSIX `rename(2)`); this project targets Linux only.
- When adding tests, use real file I/O with `t.TempDir()` for storage tests — do not mock `os`.
- The branch protection on `main` requires a pull request; never push directly to `main`.
