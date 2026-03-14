# Tasks

Implementation tasks for the LLM Council Go backend, derived from tech-lead analysis. Ordered by priority.

---

## Critical

---

# Validate Config at Startup

**Priority:** Critical

**Status:** Resolved

## Description

### Goal

Fail fast during startup when required configuration is absent, rather than starting the server and failing at first request with an opaque error.

### Files

- `internal/config/config.go`
- `cmd/server/main.go`

### Context

`config.Load()` currently returns a `*Config` without validating any fields. The most critical field is `OpenRouterAPIKey`: if it is empty, every LLM call fails with a 401 from OpenRouter, but the failure only surfaces when the first request arrives. The server MUST refuse to start when required fields are missing.

A `Validate() error` method MUST be added to `Config`. `main.go` MUST call it immediately after `config.Load()` and call `log.Fatalf` if validation fails. No other behaviour changes are in scope for this task.

### Steps

1. Add a `Validate() error` method to `Config` in `internal/config/config.go`.
2. In `Validate`, return a descriptive error if `OpenRouterAPIKey` is empty.
3. Optionally validate that `CouncilModels` is non-empty and that `Port` is a non-empty string.
4. In `cmd/server/main.go`, call `cfg.Validate()` after `config.Load()` and call `log.Fatalf` on error.

## Estimate

20 min

## Exit Condition

- Running `go run ./cmd/server` with `OPENROUTER_API_KEY` unset prints a clear fatal error message and exits with a non-zero status code.
- Running with a valid key starts the server normally.
- `config.Validate()` has at minimum one test case covering the empty-key scenario.

---

# Return Error from Stage3SynthesizeFinal

**Priority:** Critical

**Status:** Open

## Description

### Goal

Propagate Stage 3 failures to callers via the standard Go error return convention instead of embedding a sentinel string in the response body.

### Files

- `internal/council/council.go`
- `internal/api/handler.go`

### Context

`Stage3SynthesizeFinal` currently has the signature `func (...) StageThreeResult`. When the OpenRouter call fails, it logs the error and returns `StageThreeResult{Response: "Error: Unable to generate final synthesis."}`. This violates Go error-handling conventions: callers cannot distinguish a real chairman answer from an error without string matching. Worse, the `sendMessageStream` handler never checks for failure, so a chairman error is silently persisted to storage as an assistant message.

The method MUST be changed to return `(StageThreeResult, error)`. All three call sites — `RunFull` in `council.go`, `sendMessage` in `handler.go`, and `sendMessageStream` in `handler.go` — MUST be updated to check the returned error and respond with an appropriate HTTP error or SSE error event.

### Steps

1. Change the signature of `Stage3SynthesizeFinal` to return `(StageThreeResult, error)`.
2. Return the error from `c.client.QueryModel` directly instead of logging it and constructing a sentinel response.
3. Update `RunFull` in `council.go` to propagate the error.
4. Update `sendMessage` in `handler.go` to check the error and call `writeJSON` with `http.StatusInternalServerError` on failure.
5. Update `sendMessageStream` in `handler.go` to check the error and call `send` with an error event on failure, without persisting a partial assistant message.

## Estimate

25 min

## Exit Condition

- `Stage3SynthesizeFinal` signature is `func (...) (StageThreeResult, error)`.
- No sentinel error strings remain in `StageThreeResult.Response`.
- Both `sendMessage` and `sendMessageStream` handle the error return and do not write a partial assistant message to storage on chairman failure.
- Existing behaviour on the success path is unchanged.

---

# Add Graceful Shutdown to HTTP Server

**Priority:** Critical

**Status:** Open

## Description

### Goal

Allow in-flight requests to complete before the process exits when a termination signal is received.

### Files

- `cmd/server/main.go`

### Context

`main.go` calls `log.Fatal(http.ListenAndServe(...))`, which gives in-flight handlers no opportunity to finish when the process is stopped. Council requests can run for two minutes or more due to LLM round-trips; a SIGTERM during that window abandons active requests and may leave conversation files in a corrupt or incomplete state (the storage layer writes via a temp file and rename, but a request killed mid-flight will not have written the final rename).

The server MUST catch `SIGINT` and `SIGTERM`, call `http.Server.Shutdown` with a context deadline, and wait for all active handlers to return before exiting. The shutdown timeout SHOULD be at least five minutes to accommodate slow LLM calls. `log.Fatal(http.ListenAndServe(...))` MUST be removed.

### Steps

1. Replace the `http.ListenAndServe` call with an `http.Server` struct setting at minimum `Addr` and `Handler`.
2. Start the server in a goroutine.
3. Use `signal.NotifyContext` (or `signal.Notify`) to block until `os.Interrupt` or `syscall.SIGTERM` is received.
4. On signal receipt, call `srv.Shutdown` with a timeout context of at least five minutes.
5. Log when shutdown begins and when it completes.

## Estimate

30 min

## Exit Condition

- Sending SIGINT to a running server prints a shutdown-started log line.
- In-flight HTTP handlers are allowed to complete before the process exits (verifiable by adding a temporary sleep in a handler).
- The process exits with status 0 after graceful shutdown completes.
- `log.Fatal(http.ListenAndServe(...))` no longer appears in `main.go`.

---

## High

---

# Define Interfaces at Package Boundaries

**Priority:** High

**Status:** Open

## Description

### Goal

Introduce narrow interfaces between the `api`, `council`, and `openrouter` packages so that each layer can be tested in isolation without real network or disk calls.

### Files

- `internal/openrouter/client.go`
- `internal/council/council.go`
- `internal/api/handler.go`

### Context

This task is a PREREQUISITE for task 8 (storage integration tests) and task 16 (handler tests). Currently all layers depend on concrete types: `Handler` holds a `*council.Council`, and `Council` holds a `*openrouter.Client`. This makes it impossible to inject fakes in tests.

Per the Dependency Inversion principle, each consumer MUST depend on an interface, not a concrete type. Interfaces SHOULD be defined near their consumer, not their implementor. Three interfaces are needed:

- `LLMClient` in `internal/council` — covering `QueryModel` and `QueryModelsParallel`, consumed by `Council`
- `CouncilRunner` in `internal/api` — covering `RunFull`, `Stage1CollectResponses`, `Stage2CollectRankings`, `Stage3SynthesizeFinal`, and `GenerateTitle`, consumed by `Handler`
- `ConversationStore` in `internal/api` — covering `Create`, `Get`, `List`, `AddMessage`, and `UpdateTitle`, consumed by `Handler`

The concrete types `*openrouter.Client`, `*council.Council`, and `*storage.Store` MUST satisfy these interfaces (verified by compile-time blank-identifier assertions). `New` constructors MUST accept the interface type rather than the concrete type.

### Steps

1. Define `LLMClient` interface in `internal/council/council.go` (or a new `internal/council/interfaces.go`).
2. Change the `Council.client` field type and the `council.New` parameter to accept `LLMClient`.
3. Add a compile-time assertion: `var _ LLMClient = (*openrouter.Client)(nil)`.
4. Define `CouncilRunner` and `ConversationStore` interfaces in `internal/api/handler.go` (or a new `internal/api/interfaces.go`).
5. Change `Handler` fields and `api.New` parameters to accept the interface types.
6. Add compile-time assertions for both interface implementations.

## Estimate

40 min

## Exit Condition

- `go build ./...` succeeds without changes to concrete type method signatures.
- `Handler` and `Council` hold interface-typed fields, not concrete pointer types.
- Compile-time interface satisfaction assertions are present for all three interfaces.
- No functional behaviour is changed.

---

# Use Background Context for Title Generation Goroutine

**Priority:** High

**Status:** Open

## Description

### Goal

Prevent the title generation goroutine from being cancelled when the HTTP request context is cancelled before it completes.

### Files

- `internal/api/handler.go`

### Context

In both `sendMessage` and `sendMessageStream`, the title generation goroutine is started with `r.Context()` (or a `ctx` derived from it). If the client disconnects before the goroutine finishes — likely for the streaming path, since the client may close the connection after receiving the final stage result — the context is cancelled and `GenerateTitle` returns `"New Conversation"` even if the model call was already in progress.

Title generation is a fire-and-forget background task. It MUST use `context.Background()` or a context derived from it with a fixed timeout, so its lifecycle is independent of the request. A timeout of 30 seconds SHOULD be applied, matching the existing `QueryModel` call inside `GenerateTitle`.

### Steps

1. In `sendMessage`, change the goroutine to pass `context.WithTimeout(context.Background(), 30*time.Second)` to `GenerateTitle` instead of `r.Context()`.
2. In `sendMessageStream`, make the same change: pass a background-derived context instead of `ctx`.

## Estimate

15 min

## Exit Condition

- `GenerateTitle` is called with a context not derived from the request context in both handlers.
- Cancelling the HTTP request (e.g. via `curl --max-time 5`) does not prevent the title from being saved to storage if the model responds within 30 seconds.

---

# Add HTTP Client Timeout to openrouter.Client

**Priority:** High

**Status:** Open

## Description

### Goal

Set a transport-level timeout on the `http.Client` inside `openrouter.Client` as a safety net against hung TCP connections that per-request context timeouts may not cover on all platforms.

### Files

- `internal/openrouter/client.go`

### Context

`openrouter.New` constructs `&http.Client{}` with no `Timeout` set. While `QueryModel` already applies a per-request context deadline via `context.WithTimeout`, a connection that has stalled at the TCP level (e.g. zero-window or half-open socket) may not be released by context cancellation on all platforms. Setting `http.Client.Timeout` adds a redundant layer of protection.

The transport-level timeout SHOULD be longer than the longest per-request context timeout used by any caller (currently 120 seconds), so it acts only as a backstop. A value of 300 seconds is appropriate. A comment MUST explain the relationship between this timeout and the per-request context deadline.

### Steps

1. In `openrouter.New`, set `httpClient: &http.Client{Timeout: 300 * time.Second}`.
2. Add a comment above the field explaining that this is a transport-level backstop and that per-request deadlines are applied separately via context.

## Estimate

10 min

## Exit Condition

- `openrouter.New` returns a client whose `httpClient.Timeout` is non-zero.
- The value is documented with an explanatory comment.
- `go build ./...` passes.

---

# Write Unit Tests for parseRankingFromText and CalculateAggregateRankings

**Priority:** High

**Status:** Open

## Description

### Goal

Establish a unit test suite for the two pure ranking functions in the `council` package that underpin the metadata returned to the frontend.

### Files

- `internal/council/council_test.go` (new file)

### Context

`parseRankingFromText` and `CalculateAggregateRankings` are pure functions with no external dependencies. They implement the ranking extraction and aggregation logic that determines which model is presented as the best answer. Bugs in these functions produce wrong rankings in the UI with no server-side error signal.

These functions are ideal for table-driven tests. No mocks or interfaces are required, making this task independent of task 4. Tests MUST cover at minimum: correct `FINAL RANKING:` section parsing, fallback behaviour when the section is absent, edge cases with empty input, and correct average rank calculation across multiple rankers.

### Steps

1. Create `internal/council/council_test.go` with `package council`.
2. Write a table-driven test for `parseRankingFromText` covering:
   - Input with a valid `FINAL RANKING:` section and numbered entries.
   - Input with `FINAL RANKING:` but bare labels (no numbering).
   - Input with no `FINAL RANKING:` marker (fallback to full-text scan).
   - Empty string input returning an empty slice.
3. Write a table-driven test for `CalculateAggregateRankings` covering:
   - Normal input with multiple rankers; verify average rank values and sort order.
   - Empty `stage2Results` slice returns an empty slice.
   - A label present in rankings but absent from `labelToModel` is silently ignored.
4. Run `go test ./internal/council/...` and confirm all tests pass.

## Estimate

45 min

## Exit Condition

- `internal/council/council_test.go` exists and compiles with `package council`.
- `go test ./internal/council/...` reports all tests passing.
- At least 6 table-driven test cases exist across the two functions.
- No mocks, network calls, or filesystem access are used.

---

# Write Integration Tests for Storage Layer

**Priority:** High

**Status:** Open

## Description

### Goal

Verify that `storage.Store` correctly creates, reads, updates, and lists conversations using real filesystem operations, and that concurrent writes to the same conversation are safe.

### Files

- `internal/storage/storage_test.go` (new file)

### Context

This task is BLOCKED BY task 4 (interface definitions). The handler tests in task 16 will depend on the `ConversationStore` interface, and verifying that `*storage.Store` satisfies that interface is easiest to confirm here alongside the behavioural tests.

`storage.Store` uses real file I/O and per-conversation `sync.Mutex` locking. These semantics cannot be verified without exercising the actual filesystem. Tests MUST use `t.TempDir()` as `dataDir` so they leave no files outside the test's temporary directory. The concurrent write test SHOULD be run with `go test -race` to confirm the mutex eliminates data races.

### Steps

1. Create `internal/storage/storage_test.go` with `package storage`.
2. Write a test for `Create`: verify returned `Conversation` fields and that the file exists on disk.
3. Write a test for `Get` on a non-existent ID: verify `nil, nil` is returned.
4. Write a test for `Get` after `Create`: verify round-trip field correctness.
5. Write a test for `AddMessage`: verify that the message count increments correctly.
6. Write a test for `UpdateTitle`: verify the title is persisted across a `Get` call.
7. Write a test for `List`: create multiple conversations and verify they are returned sorted by `CreatedAt` descending.
8. Write a concurrent `AddMessage` test: launch 10 goroutines each adding one message to the same conversation ID; after all goroutines finish, verify exactly 10 messages are stored.
9. Run `go test -race ./internal/storage/...` and confirm all tests pass with no race conditions.

## Estimate

60 min

## Exit Condition

- `internal/storage/storage_test.go` exists and compiles.
- `go test -race ./internal/storage/...` reports all tests passing with no data race detected.
- All 8 test scenarios described in the Steps are present.
- No test writes files outside `t.TempDir()`.

---

# Write Handler Tests Using Mock Interfaces

**Priority:** High

**Status:** Open

## Description

### Goal

Test the HTTP handler layer in isolation using in-process fakes that implement the interfaces defined in task 4, with no network or filesystem calls.

### Files

- `internal/api/handler_test.go` (new file)

### Context

This task is BLOCKED BY task 4 (interface definitions at package boundaries). Once `Handler` accepts `CouncilRunner` and `ConversationStore` interfaces, tests can inject simple struct-based fakes that return canned data.

Handlers MUST be tested via `net/http/httptest`: use `httptest.NewRecorder` for non-streaming endpoints and `httptest.NewServer` for the SSE streaming endpoint. Fakes SHOULD be defined as unexported structs in the test file itself to avoid polluting the package's exported surface.

### Steps

1. Create `internal/api/handler_test.go` with `package api`.
2. Define `fakeStore` implementing `ConversationStore` and `fakeCouncil` implementing `CouncilRunner`, with configurable return values.
3. Write a test for `GET /`: assert HTTP 200 and `{"status":"ok",...}` body.
4. Write a test for `POST /api/conversations`: assert HTTP 201 and that the returned body contains an `id` field.
5. Write a test for `GET /api/conversations/{id}` with a known ID: assert HTTP 200 and correct body.
6. Write a test for `GET /api/conversations/{id}` with an unknown ID: assert HTTP 404.
7. Write a test for `POST /api/conversations/{id}/message` with a valid request: assert HTTP 200 and council result fields in the response.
8. Write a test for `POST /api/conversations/{id}/message` where `RunFull` returns an error: assert HTTP 500.
9. Run `go test ./internal/api/...` and confirm all tests pass.

## Estimate

60 min

## Exit Condition

- `internal/api/handler_test.go` exists and compiles.
- `go test ./internal/api/...` reports all tests passing.
- No test makes real network calls or accesses the filesystem.
- All 8 test scenarios described in the Steps are present.
- `fakeStore` and `fakeCouncil` are defined only in the test file.

---

## Medium

---

# Move CalculateAggregateRankings Out of Handler

**Priority:** Medium

**Status:** Open

## Description

### Goal

Remove the direct call to `council.CalculateAggregateRankings` from the handler layer so that ranking computation is fully encapsulated within the `council` package.

### Files

- `internal/council/council.go`
- `internal/api/handler.go`

### Context

`sendMessageStream` in `handler.go` calls `council.CalculateAggregateRankings(stage2, labelToModel)` directly. This means the handler contains business logic and is tightly coupled to an internal council computation, violating the Single Responsibility principle. The non-streaming `RunFull` path already encapsulates this call inside `council.go`; the streaming path should follow the same pattern.

The fix SHOULD introduce a council method (e.g. `RunStages`) that returns all intermediate results including aggregate rankings, eliminating the need for the handler to call any package-level `council.*` function. Once no external caller uses `CalculateAggregateRankings`, it SHOULD be made unexported.

### Steps

1. Evaluate whether a `RunStages` method returning all intermediate data is appropriate, or whether `sendMessageStream` should call `RunFull` and decompose the result.
2. Implement the chosen approach in `council.go`.
3. Update `sendMessageStream` in `handler.go` to remove the direct call to `council.CalculateAggregateRankings`.
4. If no callers outside the `council` package remain, rename `CalculateAggregateRankings` to `calculateAggregateRankings`.
5. Run `go build ./...` to verify no compilation errors.

## Estimate

30 min

## Exit Condition

- `handler.go` contains no direct calls to `council.CalculateAggregateRankings` or any other `council` package-level function.
- `CalculateAggregateRankings` is either unexported or exclusively called from within the `council` package.
- All existing functionality is preserved.
- `go build ./...` passes.

---

# Extract Prompts to Constants in Council Package

**Priority:** Medium

**Status:** Open

## Description

### Goal

Move the large inline prompt format strings in `council.go` to named variables so they are easier to review, test, and modify independently of the surrounding orchestration logic.

### Files

- `internal/council/council.go`

### Context

`Stage2CollectRankings`, `Stage3SynthesizeFinal`, and `GenerateTitle` each contain multi-line `fmt.Sprintf` format strings embedded directly in function bodies. These strings are interleaved with control flow, making them hard to read in isolation and impossible to test without executing the surrounding logic. Extracting them to named variables separates prompt data from orchestration code without adding any abstraction.

Named variables SHOULD be placed in a new `internal/council/prompts.go` file to avoid cluttering `council.go`. Format strings that contain `%s` placeholders MUST be declared as `var` (not `const`). Function signatures MUST NOT change.

### Steps

1. Create `internal/council/prompts.go` in the `council` package.
2. Extract the ranking prompt format string from `Stage2CollectRankings` into a named variable (e.g. `rankingPromptTemplate`).
3. Extract the chairman synthesis prompt format string from `Stage3SynthesizeFinal` into a named variable (e.g. `chairmanPromptTemplate`).
4. Extract the title generation prompt format string from `GenerateTitle` into a named variable (e.g. `titlePromptTemplate`).
5. Replace the inline string literals in `council.go` with references to these variables.
6. Run `go build ./...` and `go test ./internal/council/...` to confirm no regressions.

## Estimate

20 min

## Exit Condition

- `internal/council/prompts.go` exists and contains all three prompt format strings.
- `council.go` contains no multi-line inline prompt string literals.
- `go build ./...` and `go test ./internal/council/...` pass without errors.

---

# Make Title Model Configurable via Environment Variable

**Priority:** Medium

**Status:** Open

## Description

### Goal

Allow the model used for title generation to be overridden via an environment variable instead of being hard-coded in source.

### Files

- `internal/config/config.go`
- `internal/council/council.go`

### Context

`GenerateTitle` in `council.go` hard-codes `"google/gemini-2.5-flash"` as the model. This is the only model reference in the codebase that bypasses `Config`, making it impossible to change without a code edit. It is also inconsistent with how `CouncilModels` and `ChairmanModel` are already configured.

A `TitleModel string` field MUST be added to `Config` and read from the `TITLE_MODEL` environment variable, with `"google/gemini-2.5-flash"` as the default. `Council` MUST receive the title model at construction time and use it in `GenerateTitle`. The hard-coded string literal in `council.go` MUST be removed.

### Steps

1. Add `TitleModel string` to `Config` in `internal/config/config.go`.
2. In `config.Load()`, read `os.Getenv("TITLE_MODEL")` with `"google/gemini-2.5-flash"` as the default.
3. Add a `titleModel string` field to the `Council` struct in `council.go`.
4. Update `council.New` to accept the title model string as a parameter and store it.
5. Replace the hard-coded `"google/gemini-2.5-flash"` in `GenerateTitle` with `c.titleModel`.
6. Update `cmd/server/main.go` to pass `cfg.TitleModel` to `council.New`.

## Estimate

20 min

## Exit Condition

- Setting `TITLE_MODEL=openai/gpt-4o-mini` causes that model to be used for title generation.
- `council.New` signature includes the title model parameter.
- The string literal `"google/gemini-2.5-flash"` does not appear in `council.go`.
- `go build ./...` passes.

---

# Log writeJSON Encoder Errors

**Priority:** Medium

**Status:** Open

## Description

### Goal

Ensure that JSON encoding errors in the `writeJSON` helper are not silently discarded.

### Files

- `internal/api/handler.go`

### Context

`writeJSON` calls `json.NewEncoder(w).Encode(v)` and discards the returned error. If encoding fails — for example due to a non-serializable value passed by a caller, or a write error on the connection — the client receives a partial or empty response body with no indication of failure. Because `w.WriteHeader` has already been called, the status code cannot be changed at that point, but the error MUST be logged so that it appears in server output and can be diagnosed.

The fix is small and self-contained. No change to the `writeJSON` signature or any of its callers is required.

### Steps

1. In `writeJSON`, assign the return value of `json.NewEncoder(w).Encode(v)` to a local variable.
2. If the error is non-nil, log it using `log.Printf("writeJSON: encode error: %v", err)`.

## Estimate

10 min

## Exit Condition

- `writeJSON` captures and logs `Encode` errors.
- The function signature is unchanged and no callers are modified.
- `go build ./...` passes.

---

# Switch to Structured Logging with slog

**Priority:** Medium

**Status:** Open

## Description

### Goal

Replace all `log.Printf` calls with `slog` structured logging to produce machine-parseable output suitable for log aggregation and alerting.

### Files

- `internal/council/council.go`
- `internal/storage/storage.go`
- `internal/api/handler.go`
- `cmd/server/main.go`

### Context

The codebase uses `log.Printf` throughout, producing unstructured text logs. `slog` has been part of the Go standard library since Go 1.21 and provides structured key-value logging at no additional dependency cost. Structured logs are necessary for filtering by conversation ID, model name, or error type in any deployed environment.

A single `slog.Logger` SHOULD be configured in `main.go` (using `slog.NewJSONHandler` targeting `os.Stdout`). The default package-level `slog` functions MAY be used as a first step if constructor injection of the logger is considered premature. All existing `log.Printf` call sites MUST be replaced with equivalent `slog.Info`, `slog.Warn`, or `slog.Error` calls that include structured key-value attributes. `log.Fatalf` in `main.go` MAY be replaced with `slog.Error` followed by `os.Exit(1)`.

### Steps

1. In `cmd/server/main.go`, configure the `slog` default handler to use `slog.NewJSONHandler(os.Stdout, nil)` before any other setup.
2. Replace `log.Printf` calls in `council.go` with `slog.Error`, passing `"model"` and `"error"` as structured attributes.
3. Replace `log.Printf` calls in `storage.go` with `slog.Warn`, passing `"path"` and `"error"` as structured attributes.
4. Replace `log.Printf` calls in `handler.go` with appropriate `slog.Error` or `slog.Warn` calls with structured attributes.
5. Run `go build ./...` and confirm no `"log"` package imports remain except where `log.Fatal` is still used.

## Estimate

40 min

## Exit Condition

- No `log.Printf` calls remain in the codebase.
- All log call sites use `slog` with at least one structured key-value attribute beyond the message string.
- `go build ./...` passes.
- A simulated model error log line includes both the model name and the error message as distinct JSON fields.

---

## Low

---

# Add staticcheck to Makefile Lint Target

**Priority:** Low

**Status:** Open

## Description

### Goal

Run `staticcheck` as part of the standard lint target to catch static analysis issues that `go vet` does not cover.

### Files

- `Makefile`

### Context

`staticcheck` is the de facto standard static analyzer for Go, catching issues such as unused parameters, deprecated API usage, incorrect format verbs, and unreachable code. It SHOULD be run alongside `go vet` in CI and during local development. If a `Makefile` does not yet exist in the project root, it MUST be created with at minimum `lint` and `test` targets.

`staticcheck` MUST be declared as a module tool dependency (e.g. in a `tools.go` file with a `//go:build tools` build tag) so that its version is pinned in `go.mod` and reproducible across environments. The lint target MUST fail the build if either `go vet` or `staticcheck` exits non-zero.

### Steps

1. Check whether a `Makefile` exists in the project root; create one if absent.
2. Add or update a `lint` target that runs `go vet ./...` followed by `staticcheck ./...`.
3. Create a `tools.go` file with `//go:build tools` importing `honnef.co/go/tools/cmd/staticcheck` to pin the version.
4. Run `go mod tidy` to update `go.sum`.
5. Verify `make lint` runs without errors on the current codebase.

## Estimate

20 min

## Exit Condition

- `make lint` runs both `go vet ./...` and `staticcheck ./...`.
- `staticcheck` is pinned as a module tool dependency.
- `make lint` exits non-zero if either tool reports an issue.

---

# Fix Minor Issues: 201 for createConversation, titleCh Scope, CORS Origins in Config

**Priority:** Low

**Status:** Open

## Description

### Goal

Resolve three small correctness and hygiene issues in the handler and config layer.

### Files

- `internal/api/handler.go`
- `internal/config/config.go`

### Context

Three unrelated minor issues are grouped here for efficiency:

1. **201 for createConversation**: `POST /api/conversations` returns `http.StatusOK` (200) but SHOULD return `http.StatusCreated` (201) per HTTP semantics for resource creation. The frontend MUST tolerate either status code before this change is deployed.

2. **titleCh scope in sendMessage**: In `sendMessage`, `titleCh` is declared as `var titleCh chan string` outside the `if isFirst` block but is only assigned inside it. The outer declaration is wider than necessary and could mislead a future reader into believing the channel is always initialised. The declaration SHOULD be moved inside the `if isFirst` block.

3. **CORS origins in config**: The allowed CORS origins (`http://localhost:5173`, `http://localhost:3000`) are hard-coded in `corsMiddleware`. These SHOULD be moved to `Config`, read from a `CORS_ORIGINS` environment variable as a comma-separated list, so they can be adjusted for production deployments without modifying source code.

### Steps

1. In `createConversation`, change `writeJSON(w, http.StatusOK, conv)` to `writeJSON(w, http.StatusCreated, conv)`.
2. In `sendMessage`, move `titleCh := make(chan string, 1)` inside the `if isFirst` block and remove the outer `var titleCh chan string` declaration.
3. Add `CORSOrigins []string` to `Config` in `internal/config/config.go`.
4. In `config.Load()`, read `CORS_ORIGINS` as a comma-separated list, defaulting to `["http://localhost:5173", "http://localhost:3000"]`.
5. Update `api.New` to accept `cfg.CORSOrigins` (or pass the full config) and use the slice in `corsMiddleware`.
6. Run `go build ./...` and verify all tests still pass.

## Estimate

30 min

## Exit Condition

- `POST /api/conversations` returns HTTP 201.
- `titleCh` is declared only within the `if isFirst` block in `sendMessage`.
- Allowed CORS origins are read from `Config` and the hard-coded list is removed from `corsMiddleware`.
- `go build ./...` passes.
