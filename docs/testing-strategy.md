# Testing Strategy — v2 Council Implementation

Defines the testing layers, tooling, and conventions for the LLM Council v2 backend.
Each layer targets a different risk surface; together they give confidence that the
deliberation pipeline is correct under concurrency, partial failure, and adversarial
LLM output.

---

## Layers at a Glance

| Layer | Scope | Tool | Key risk |
|-------|-------|------|----------|
| Unit | Pure functions, concurrency | `go test -race` | Logic errors, race conditions |
| Contract | HTTP wire shape | `net/http/httptest` | Schema drift vs. OpenRouter API |
| Integration | Real filesystem I/O | `t.TempDir()` | Persistence correctness |
| Property | Numeric invariants | `go test -count=N` | Invariant violations at boundary values |

---

## 1. Unit Tests

Unit tests live in the same package under `_test.go` files. They use only fake
implementations — no network, no disk.

### 1.1 Council logic (`internal/council/`)

**Seam:** `LLMClient` interface. All stage methods accept a `*Council` whose `client`
field is a `mockLLMClient` with a scriptable `complete` function.

**Coverage targets:**

| File | What is tested |
|------|----------------|
| `council_test.go` | `checkQuorum` (N=2/4, override, edge cases), `assignLabels` (round-trip, letter format), `BuildStage3Prompt` (W-guidance thresholds, structured attribution) |
| `runner_test.go` | `runStage1` (all succeed, partial failure, context cancellation, empty choices), `runStage2` (all succeed, parse failure, unknown labels dropped, empty/null/missing rankings, LLM failure, `json_object` format flag), `runStage3` (success, client error, empty choices, chairman model routing, temperature and stage-1 content forwarding), `RunFull` (unknown type, quorum failure, no stage2/3 events after failure, happy-path event sequence, stage2 payload shape and model-name translation) |
| `rankings_test.go` | `CalculateAggregateRankings` (full agreement W=1, no agreement W=0, midrank imputation, all missing, empty inputs, single judge, manual two-judge calculation) |

**Concurrency:** `runStage1` and `runStage2` fan out goroutines into a pre-allocated
slice (index-keyed, no mutex). Tests are run with `-race` to detect any data races.

**Mock pattern:**

```go
type mockLLMClient struct {
    complete func(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

func (m *mockLLMClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
    return m.complete(ctx, req)
}
```

The closure receives the full `CompletionRequest`, letting tests inspect which model,
temperature, and `ResponseFormat` were sent without additional instrumentation.

**Dynamic label extraction:** `RunFull` assigns random labels via `rand.Perm`. The
`fullPipelineClient` helper extracts the actual labels from the stage-2 prompt text
(`labelsFromPrompt`) so mock reviewers return valid rankings without predicting the
assignment:

```go
func labelsFromPrompt(content string) []string {
    var labels []string
    for _, line := range strings.Split(content, "\n") {
        if strings.HasPrefix(line, "## Response ") {
            labels = append(labels, strings.TrimPrefix(line, "## "))
        }
    }
    return labels
}
```

### 1.2 API handler (`internal/api/`)

**Seams:** `Storer` and `Runner` interfaces. `mockStorer` and `mockRunner` are defined
in `handler_test.go`; each method delegates to an optional function field, defaulting
to a no-op success on nil.

**Coverage targets:** request validation, UUID format checking, SSE event sequencing,
quorum error → SSE error event mapping, pre-SSE HTTP 400/404/500 paths.

---

## 2. Contract Tests

Contract tests verify that the OpenRouter client sends the correct HTTP wire shape and
parses responses correctly. They use `net/http/httptest.NewServer` — a real TCP
listener, no mocking.

**File:** `internal/openrouter/client_test.go`

**Coverage targets:**

- Required headers (`Authorization: Bearer`, `HTTP-Referer`, `X-Title`) present on
  every request
- Request body serialises `model`, `messages`, `temperature`, and optional
  `response_format` correctly
- Successful response deserialises to `CompletionResponse.Choices[0].Message.Content`
- HTTP 4xx/5xx returns a descriptive error
- Timeout / context cancellation propagates as an error

**Why httptest, not a mock:** The OpenRouter API uses standard OpenAI-compatible JSON
over HTTP. Testing against a real TCP server catches serialisation bugs (missing
`omitempty`, wrong field names) that an in-process mock would silently absorb.

---

## 3. Integration Tests

Integration tests exercise the storage layer against a real filesystem. They use
`t.TempDir()` which Go automatically cleans up after each test, giving full isolation
without manual teardown.

**File:** `internal/storage/storage_test.go`  
**Package:** `storage_test` (black-box; no access to unexported fields)

**Coverage targets:**

- `CreateConversation` → `GetConversation` round-trip (ID, title, timestamps preserved)
- `SaveUserMessage` / `SaveAssistantMessage` append correctly and are visible to
  subsequent `GetConversation` calls
- `ListConversations` returns only metadata (not full message history)
- `GetConversation` with unknown ID returns a typed `ErrNotFound`
- Concurrent writes to the same conversation do not corrupt the file

**Filesystem layout under test:** each `t.TempDir()` creates an isolated directory;
the `Store` writes one JSON file per conversation. Tests can inspect raw files via
`os.ReadFile` when validating persistence shape.

---

## 4. Property-Based Tests

Property tests verify numeric invariants that hold for all valid inputs, not just the
specific cases covered by example-based tests.

### 4.1 Kendall's W bounds

**Invariant:** `CalculateAggregateRankings` must always return `W ∈ [0, 1]`.

Covered by repeated calls with randomised `StageTwoResult` slices:

```go
// Run with: go test -count=1000 ./internal/council/
func TestW_AlwaysInBounds(t *testing.T) {
    for range 1000 {
        n := rand.Intn(5) + 2          // 2–6 candidates
        k := rand.Intn(4) + 1          // 1–4 judges
        stage2 := randomStage2(k, n)
        labels := labelsForN(n)
        _, W := CalculateAggregateRankings(stage2, labels)
        if W < 0 || W > 1+1e-9 {
            t.Errorf("W=%.6f out of [0,1]", W)
        }
    }
}
```

### 4.2 Rank-sum consistency

**Invariant:** the sum of `AverageRank` values across all returned `RankedModel`
entries equals `(n+1)/2 * n` (the expected total for a balanced ranking). This
catches off-by-one errors in midrank imputation.

### 4.3 Race detector sweep

The race detector is a property test in disguise: it asserts the concurrent fan-out
in `runStage1` and `runStage2` contains no happens-before violations under any
goroutine schedule. Run with:

```bash
go test -race ./...
```

This must pass before every PR merge.

---

## 5. Running the Suite

```bash
# Fast: all tests, no race detector
go test ./...

# Full: with race detector (required before merging)
go test -race ./...

# Single package
go test -race ./internal/council/

# Verbose with property iteration
go test -race -count=1000 -v ./internal/council/ -run TestW_
```

---

## 6. What Is Not Tested Here

| Category | Rationale |
|----------|-----------|
| End-to-end against live OpenRouter API | Non-deterministic, rate-limited, costs tokens. Covered by manual smoke tests only. |
| Frontend React components | Separate layer; covered by `npm run lint` and Playwright (deferred to LCCP Full). |
| LCCP state machine transitions | State machine is defined in `docs/council-research-synthesis.md §3` but not yet implemented. Property tests for invariants (§4 above) will expand as the state machine is built. |

---

## Related Documents

- `docs/council-research-synthesis.md §8` — Go implementation patterns and interface design
- `docs/council-research-synthesis.md §12` — Infrastructure prerequisites (handler tests)
- `docs/api.md` — SSE error shapes that handler tests must verify
