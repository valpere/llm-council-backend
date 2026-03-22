---
name: project_architecture
description: LLM Council Go backend — module map, key design decisions, established conventions
type: project
---

Go backend for LLM Council, a 3-stage multi-LLM deliberation system.

**Why:** Python/FastAPI original was rewritten to Go for performance and deployment simplicity.
**How to apply:** Architecture decisions must remain consistent with the modular monolith pattern already established.

## Module layout

```
cmd/server/main.go            — wire-up only; config → openrouter → council → storage → api
internal/config/config.go     — Config struct, Load() from env; no validation today
internal/openrouter/client.go — QueryModel() / QueryModelsParallel(); concrete type, no interface
internal/council/types.go     — domain types: StageOneResult, StageTwoResult, StageThreeResult, Metadata, Result
internal/council/council.go   — Council struct (concrete dep on *openrouter.Client); Stage1/2/3, RunFull, GenerateTitle
internal/storage/storage.go   — JSON file store; atomic writes; per-conv sync.Mutex via sync.Map
internal/api/handler.go       — HTTP handlers + SSE streaming; CORS middleware; direct dep on *council.Council and *storage.Store
```

## Established conventions

- Atomic file writes: write to `{id}.json.tmp`, then `os.Rename`
- Per-conversation locking: `sync.Map` of `*sync.Mutex`, acquired via `lockConv(id)`
- UUID-validated IDs before any file path construction (path traversal prevention)
- Request body capped at 1MB via `http.MaxBytesReader`
- CORS: only localhost:5173 and localhost:3000 allowed
- Data dir permissions: 0700; file permissions: 0600
- SSE events: `data: {...}\n\n` with `type` field; no SSE `event:` line
- `math/rand` auto-seeded (Go 1.20+), no explicit seed needed
- External deps: only `github.com/google/uuid` and `github.com/joho/godotenv`
- `go vet ./...` is the linting step (no staticcheck/golangci-lint yet)

## Key design decisions

- labelToModel mapping is ephemeral — not persisted, only in API response
- Stage 2 is capped at 26 council members (A-Z label limit)
- GenerateTitle uses a hardcoded model (`google/gemini-2.5-flash`) regardless of configured council
- Title generation runs concurrently with RunFull/Stage1 to avoid blocking
- No graceful shutdown; `http.ListenAndServe` called directly
- No structured logging; stdlib `log` package only
- No tests exist yet; copilot-instructions.md says to use real file I/O (temp dirs), not mocks
