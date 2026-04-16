# API Error Contract — v2

Defines error handling across all v2 API endpoints. Closes the gap identified in issue #69.

---

## Scope

This contract covers **LCCP Core** (v2 initial build). BestSoFar partial results, budget
exhaustion, schema violation, and controller-error categories are deferred to LCCP Full.

---

## 1. Pre-SSE HTTP errors

Returned before the SSE stream is established (i.e., before `Content-Type: text/event-stream`
is written), so a proper HTTP status code is possible.

### Shape

```json
{"error": "<human-readable message>"}
```

### Status codes by failure

| Failure | Status | `error` message |
|---------|--------|----------------|
| Invalid conversation ID format | `400` | `"invalid conversation id"` |
| Malformed or missing request body | `400` | `"invalid request body"` |
| Conversation not found | `404` | `"not found"` |
| Storage failure (pre-pipeline operations) | `500` | `"internal server error"` |
| SSE streaming not supported by server | `500` | `"streaming not supported"` |

---

## 2. SSE error events

Once SSE is established (HTTP `200`, `Content-Type: text/event-stream`), the HTTP status
code is locked. Errors are communicated as SSE events. The stream terminates immediately
after the error event; no `complete` event follows.

### Shape

```json
{"type": "error", "message": "<human-readable message>"}
```

### LCCP Core failure events

| Failure | `message` |
|---------|-----------|
| Stage 1 quorum not met | `"council quorum not met"` |
| Stage 3 Chairman LLM failure | `"internal server error"` |
| Storage failure saving assistant message | `"internal server error"` |

### Quorum failure detail

When Stage 1 returns fewer than M_min successful responses (`QuorumError{Got, Need}`),
the handler logs `Got`/`Need` at `WARN` level, emits the SSE error event, and returns.
No `stage2_complete` or `stage3_complete` events are emitted before the error.
No assistant message or stage outputs from the failed run are persisted. The user
message may already have been saved before SSE began.

---

## 3. Partial results (BestSoFar)

**Not returned in v2 Core.** On any pipeline failure the client receives only the SSE
error event; no assistant message or stage outputs from the failed run are persisted.
The user message may already have been saved before SSE began.

BestSoFar tracking (return the best stage-N result reached before the failure) is
deferred to LCCP Full conformance.

---

## 4. Deferred failure categories (LCCP Full)

When the following failure categories are implemented, the SSE error shape will extend
to include `reason_code` and `recoverable` for programmatic handling:

```json
{
  "type": "error",
  "reason_code": "<code>",
  "message": "<human-readable message>",
  "recoverable": false
}
```

| `reason_code` | Category | Notes |
|---------------|----------|-------|
| `insufficient_quorum` | Participant | Promoted from LCCP Core (message-only) to structured |
| `budget_exhausted` | Controller | Token budget cap exceeded |
| `schema_violation` | Finalization | Stage 2/3 output violates schema bounds |
| `controller_error` | Controller | Illegal LCCP state machine transition — always fatal |
| `evaluation_invalid` | Evaluation | Stage 2 evaluation output is invalid, incomplete, or unusable |
| `aggregation_failed` | Aggregation | Ranking/aggregation computation failure |
| `participant_timeout_exhaustion` | Participant | All per-participant retries consumed |

`reason_code` values are derived from the failure schema in
`docs/council-research-synthesis.md §4` (`failure.reason_code` enumeration);
the Category column maps to the taxonomy in §6.

---

## Related documents

- `docs/frontend/streaming.md` — SSE event sequence and frontend handling for `error` events
- `docs/frontend/api-contract.md` — full REST API shape (request/response bodies)
- `docs/council-research-synthesis.md §6` — LCCP failure taxonomy and severity levels
- `docs/council-research-gaps.md §1.5` — quorum enforcement decision
