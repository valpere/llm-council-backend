---
name: known_issues
description: Prioritized improvements and fixes identified in initial codebase analysis (2026-03-14)
type: project
---

Initial analysis completed 2026-03-14. Issues grouped by theme.

**Why:** Recorded so future sessions can pick up implementation work without re-analyzing.
**How to apply:** Work down this list in severity order. Do not add new abstractions without checking this list first for related issues.

## Critical

- **No API key validation at startup** — config/config.go: if OPENROUTER_API_KEY is empty the server starts silently and every request fails with a cryptic OpenRouter 401. Add a validation step in main.go before starting the HTTP server.
- **Stage3SynthesizeFinal never returns an error** — council/council.go: signature returns StageThreeResult (no error), swallows the error into a string. Callers (RunFull, handler) cannot distinguish a real error from a valid "error:" response. Change the signature to return (StageThreeResult, error).
- **No graceful shutdown** — cmd/server/main.go: `http.ListenAndServe` is called with no shutdown handling. A SIGTERM/SIGINT kills in-flight 3-stage requests mid-execution (including mid-SSE stream), leaving the conversation in a partially-written state. Add `http.Server` with `Shutdown` on signal.

## High

- **Concrete type coupling breaks testability** — api/handler.go, council/council.go: Handler depends on `*council.Council` and `*storage.Store` as concrete types, not interfaces. Council depends on `*openrouter.Client` as a concrete type. Nothing can be tested without real network calls. Define interfaces at each consumer boundary.
- **No HTTP client timeout** — openrouter/client.go: `&http.Client{}` has no global timeout. Per-request context timeouts exist, but if the context is already cancelled the idle connection pool may still hold sockets open. Set `http.Client.Timeout` as a backstop (e.g., 150s to exceed the 120s per-request timeout).
- **Title generation goroutine leaks on context cancellation** — api/handler.go sendMessageStream: if the client disconnects after Stage 1 but before the title goroutine finishes, the goroutine blocks on `<-titleCh` then tries to call `h.store.UpdateTitle` after the request is gone. The goroutine itself finishes, but context propagation to GenerateTitle is via `r.Context()` which is cancelled — however UpdateTitle is still attempted. More importantly, if sendMessage is cancelled mid-flight, the title goroutine runs with a cancelled context and blocks on its own `<-titleCh` receive. Use a background context with a separate timeout for title generation.
- **Handler.sendMessage returns 200 for createConversation** — api/handler.go: `createConversation` responds with `http.StatusOK` (200) instead of `http.StatusCreated` (201). Violates REST conventions.
- **Config.Load() has no validation and no error return** — config/config.go: returns `*Config` without reporting missing required fields. Callers cannot distinguish misconfiguration from empty string. Add a `Validate() error` method or change Load to `Load() (*Config, error)`.

## Medium

- **CalculateAggregateRankings is exported without a clear consumer reason** — council/council.go: the function is exported and called directly from api/handler.go (not via Council methods). This bypasses the Council abstraction and creates a coupling point between the api and council packages. It should be called inside Stage2CollectRankings or RunFull, not leaked to the handler.
- **Duplicate request decoding logic** — api/handler.go: `sendMessage` and `sendMessageStream` are nearly identical in their first ~25 lines (decode body, get conv, check nil, isFirst). DRY violation; extract a helper.
- **`list` scans all files on every request** — storage/storage.go: List() reads and unmarshals every JSON file in the data dir. With hundreds of conversations this becomes slow and memory-intensive. Consider an in-memory index or a separate metadata file updated on Create/UpdateTitle.
- **Hardcoded title-generation model** — council/council.go line 170: `google/gemini-2.5-flash` is hardcoded, independent of the configured council. If that model is removed from OpenRouter, title generation silently breaks. Move this to config (e.g., `TitleModel` field with a sensible default).
- **`log.Printf` is the only observability** — entire codebase: no structured logging, no request IDs, no timing, no way to correlate logs to a specific conversation or request. Adopt `log/slog` (stdlib since Go 1.21) with at minimum `conversation_id` and `model` fields.
- **`json.NewEncoder(w).Encode(v)` silently drops write errors** — api/handler.go: writeJSON's encoder error is ignored. On a dropped connection this is harmless but it hides bugs. Assign and log the error.
- **Stage 2 peer-ranking prompt is embedded as a raw string literal** — council/council.go: a 30-line prompt is inline in Stage2CollectRankings. This mixes data with logic. Extract prompts to named constants or a prompts.go file.
- **`sendMessageStream` does not send a `stage3_start` event before the call** — api/handler.go line 249: SSE docs say `stage3_start` should be emitted; the code emits it correctly but the `stage2_start` and `stage3_start` events are missing for the non-streaming `sendMessage` path (irrelevant there, but the asymmetry should be documented).

## Low

- **No `make lint` with a real linter** — Makefile: `make lint` is just `go vet`. Adding `staticcheck` or `golangci-lint` (with a minimal config) would catch issues `go vet` misses. Low priority but worth noting.
- **No tests** — entire codebase: zero test files. The copilot instructions acknowledge this. At minimum: unit tests for `parseRankingFromText`, `CalculateAggregateRankings`, and storage CRUD (with real temp dirs).
- **`validID` regex compiled at package level** — storage/storage.go: this is actually fine (package-level compiled regex is idiomatic Go), but worth noting for consistency with the rest of the analysis.
- **`Conversation.CreatedAt` is a string (RFC3339), not `time.Time`** — storage/storage.go: string comparison `metas[i].CreatedAt > metas[j].CreatedAt` works for RFC3339 lexicographically, but using `time.Time` would be more correct and enable duration calculations. Low-priority refactor.
- **CORS allowed origins are hardcoded** — api/handler.go: localhost:5173 and localhost:3000 are compile-time constants. Fine for dev-only use, but if this ever needs a configurable prod origin, it will require code changes. Add a `CORSOrigins []string` to Config.
- **`sendMessageStream` `titleCh` channel is always created even when `!isFirst`** — api/handler.go: the channel is created unconditionally but only written to when `isFirst`. If `!isFirst`, the goroutine is never started so the channel is never read, but `<-titleCh` at line 255 is gated on `isFirst`, so there is no deadlock. Minor code clarity issue; could initialize to nil and skip the select entirely.
