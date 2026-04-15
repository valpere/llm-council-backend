# LLM Council — Design Decisions and Open Questions

Things the research synthesis does not resolve, gaps to design intentionally in the new
implementation, and ambiguities that need answers before building.

The previous implementation is preserved on `archive/v1` for reference.

---

## Resolved decisions

### Context model — stateless per query

Each question is a new, independent council run with no memory of prior turns. A
conversation record exists for UI history only; it does not feed back into the council.
This is intentional for the first stage.

---

### Consensus metric — Kendall's W for rank-order strategies

Peer review (Stage 2 produces ordered rankings) maps directly to Kendall's W — no
external embedding calls, measures exactly rank agreement. For free-text strategies
(Debate, MoA), LLM-as-Judge is the v1 default (no embedding infrastructure required).
Cosine similarity is deferred until free-text strategies are added.

The W-to-prose translation from archive/v1 is carried forward:
- W ≥ 0.70 → "synthesize confidently"
- 0.40 ≤ W < 0.70 → "acknowledge alternatives"
- W < 0.40 → "present multiple perspectives"

---

### Self-evaluation paradox — anonymization is sufficient mitigation for v1

Separate ranker models add cost and operational complexity with uncertain benefit.
Mitigation: shuffled label assignment (`rand.Perm`) per request so no model is
systematically "Response A". Self-identification is probabilistic, not deterministic.
Separate ranker pools are a named council type variant, not a v1 requirement.

---

### Council type scope — model sets only for v1; strategy abstraction deferred

A "council type" is a named configuration: a fixed strategy (Karpathy peer-review)
combined with a configurable model set and parameters. Multiple council types with
different model sets require zero interface changes. Strategy variants require a dispatch
layer and are out of scope until the peer-review design is proven.

---

### LCCP first-stage scope — Core conformance, single round, no REFINE loop

Effective state machine for v1:
```
INIT → GENERATE → VALIDATE_GENERATION → EVALUATE → VALIDATE_EVALUATION
     → AGGREGATE → DECIDE → FINALIZE → TERMINATE  (+ FAIL branch at any node)
```

Explicitly deferred: REFINE loop, BestSoFar tracking, fallback finalization modes
(synthesize_top_k → select_best → fallback_best_so_far). Target: Core conformance level.
Robust and Auditable conformance are post-v1. This is the scope-limiting decision that
drives the entire architecture.

---

### Quorum enforcement — M_min = ⌈N/2⌉ + 1, minimum 2; failure → error

If Stage 1 returns fewer than M_min successful responses, return an explicit error to the
caller. A single-model Stage 2 ranking is meaningless (a model ranking its own answer).
Within-quorum partial failures (some models fail but M_min succeed) are tolerated.
The quorum threshold is a council type configuration parameter, not hardcoded.

---

### Metadata persistence — store full metadata with every assistant message

Schema addition to the assistant message:
```json
{
  "role": "assistant",
  "stage1": [...],
  "stage2": [...],
  "stage3": {...},
  "metadata": {
    "council_type": "default",
    "label_to_model": {"Response A": "model-x", ...},
    "aggregate_rankings": [...],
    "consensus_w": 0.72
  }
}
```

Old conversation files (no `metadata` field) fail gracefully via zero-value defaults
in Go struct unmarshalling.

---

### Structured JSON output for Stage 2 rankings

Use `response_format: json_object` on OpenRouter for Stage 2 ranking responses.
Schema: `{"rankings": ["Response C", "Response A", "Response B", "Response D"]}`.
If JSON parsing fails: log `slog.Warn`, treat as missing ranking (existing midrank
imputation in Kendall's W handles it). Replaces the silent regex failure mode from v1.

---

### Trust weighting — deferred; uniform weights in v1

Trust weighting requires a calibration mechanism (ground-truth oracle or historical
performance data) that doesn't exist. Uniform weights are the correct default. Defer
until an evaluation framework exists.

---

### Strategy vs. council type — distinct concepts in code and API

- **Strategy:** A deliberation algorithm (`PeerReview`, `MajorityVoting`, `MoA`). Pure
  behavior, no model specifics. Typed constant/enum in Go.
- **Council type:** A named, user-selectable configuration combining strategy + model
  set + parameters (`"default"`, `"expert"`, `"fast"`). User-facing.

In the API, the field is `council_type` (string name). Strategy is resolved server-side
from the council type registry. Users never specify a strategy directly.

---

### Chairman input — parsed/structured rankings, not raw Stage 2 prose

The Chairman receives parsed rankings formatted as structured attribution:
"Model X ranked these responses: 1st: Response C, 2nd: Response A, ...". The Chairman
prompt is constructed from Go structs, not by concatenating raw LLM output. With
structured JSON output in Stage 2 (see above), the ranking content contains no
user-controlled text — only server-assigned labels.

This resolves the internal synthesis contradiction: §2.5 (pass raw output) vs. §7/§9.9
(pass only validated summary). Middle ground: parsed structure, no full sanitization pass.

---

### Council type selection API shape — field in POST request body

```json
POST /api/conversations/{id}/message
{"content": "What is the best sorting algorithm?", "council_type": "default"}
```

`council_type` is optional; defaults to server-configured default. Server-wide config
only (archive/v1 approach) is inadequate once multiple council types exist. Query
parameters are less discoverable and harder to validate.

---

### Stage 3 graceful non-synthesis — deferred to post-v1

The W-to-prose translation already guides the Chairman toward presenting perspectives
rather than synthesizing when W < 0.40. A first-class "council could not reach a
synthesis" outcome requires the REFINE loop and LCCP fallback chain — both explicitly
out of scope (see LCCP scope decision). Document as a known v1 limitation.

---

### Streaming architecture — stage-completion events only; no intra-stage token streaming

Token-by-token streaming within a stage requires threading a streaming callback through
the entire council pipeline interface. The SSE event-per-stage model provides meaningful
progress feedback without this interface complexity. Intra-stage token streaming is a
valid post-v1 enhancement.

---

### Storage interface — pluggable Storer from day one; JSON backend is v1 only

The `Storer` interface must be designed to allow replacing the JSON backend without
rewriting handlers. The JSON file backend is v1 only — `List()` is O(n) on disk and
does not scale. Interface design must not leak JSON-specific assumptions.

---

### OpenRouter model lifecycle — model IDs in configuration, not code

Council type registry stores model IDs as configuration strings updatable without
redeployment. The implementation must not hardcode model IDs in source code.

---

### CalculateAggregateRankings — package-level function, not on Runner interface

In archive/v1 this lived on the `Runner` interface, forcing every strategy to satisfy
it regardless of whether it produces rankings. In v2 it is a package-level function
(or belongs to a separate ranking/metrics layer).

---

### Infrastructure prerequisites — confirmed requirements for any v2 build

The following are prerequisites, not optional niceties:
- Config validation at startup (fail fast on missing API key)
- HTTP client timeout on the OpenRouter client
- Graceful shutdown for the HTTP server
- Structured logging (`slog`)
- Handler tests using mock interfaces

---

## Remaining open questions

### "Preserve unique insights" is not operationalised

The synthesis repeatedly recommends "explicitly prompt synthesis to preserve unique
insights." Neither the synthesis nor any source material defines what makes an insight
"unique" or how to distinguish a fringe claim from a minority-but-correct insight.

**Current position:** The Chairman prompt includes an explicit instruction to surface
minority perspectives that appear well-reasoned even if not consensus — particularly when
W < 0.40. Formal operationalization (defining "unique" algorithmically, tracking insight
provenance) is deferred until an evaluation framework exists to validate any criterion.

**What needs answering before this can be closed:** A concrete rubric or test for
distinguishing "unique minority insight" from "fringe claim," or an explicit decision
that this is permanently a prompt-level heuristic.
