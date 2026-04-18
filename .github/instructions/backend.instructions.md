---
applyTo: "**/*.go"
---

# Go backend review rules

## Layer boundaries

- `internal/api` must not import `internal/storage`, `internal/openrouter`, or `internal/council` directly — it must use interfaces.
- `internal/council` must not import `net/http`, `internal/storage`, or `internal/api`.
- `internal/storage` must not import `net/http`, `internal/council`, or `internal/openrouter`.
- `cmd/server/main.go` is the only place that wires concrete types to interfaces.

## Errors

- Never swallow errors silently. `_ = err` and empty `if err != nil {}` blocks are always wrong.
- Log errors with `slog` at the call site before returning or responding.
- HTTP handlers must write an error response (`writeJSON` or `http.Error`) before returning on every error path.

## Interfaces

- Define interfaces at the consumer boundary, not in the implementation package.
- Add a compile-time assertion for every new interface implementation: `var _ MyInterface = (*MyImpl)(nil)`.

## Tests

- Use table-driven tests with `t.Run` and descriptive subtest names.
- Storage tests must use real file I/O with `t.TempDir()` — never mock `os`.
- No global state mutation between tests.
- `t.Cleanup` (not `defer`) for teardown inside subtests.

## Concurrency

- Every read-modify-write on a conversation in `internal/storage` must hold the per-conversation mutex.
- Goroutines spawned in handlers must use a context derived from the request (or `context.Background()` for fire-and-forget work that must outlive the request, documented with a comment explaining why).

## HTTP specifics

- Apply `http.MaxBytesReader(w, r.Body, 1<<20)` before decoding request bodies.
- Validate UUID path parameters before passing to storage.
- Set `Content-Type` before writing the status code.
- SSE events must follow `data: {...}\n\n` format with a `type` field; no `event:` line.

## Style

- No comments unless the **why** is non-obvious. Never restate what the code does.
- No magic numbers — use named constants.
- No new external dependencies beyond `go.mod` without a stated reason.
