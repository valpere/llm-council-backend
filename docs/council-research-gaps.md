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

## 1. Things the synthesis does not address — design intentionally

### 1.1 Consensus metric: Kendall's W vs. embedding cosine similarity

The synthesis covers cosine similarity at length but never mentions Kendall's coefficient
of concordance (W). These solve different problems:

- **Kendall's W** — measures agreement across multiple *rank orderings*. Correct when Stage
  2 produces ranked lists. Requires no external embedding calls. The archive/v1
  implementation uses this.
- **Cosine similarity** — measures *text content* similarity. Correct for strategies where
  models produce free-text answers with no ranking (Debate, MoA). Requires an embedding
  API or local model.

**Design decision required:** Which strategies will be supported in v1, and therefore which
consensus metric(s) are needed?

The archive/v1 implementation also translated W into prose for the Chairman prompt via a
threshold table (W ≥ 0.70 → "synthesize confidently", 0.40–0.70 → "acknowledge
alternatives", < 0.40 → "present multiple perspectives"). This W-to-prose pattern avoids
passing raw floats to the LLM and is worth carrying forward.

### 1.2 The self-evaluation paradox in Stage 2

The Karpathy peer-review design uses the **same models** as both generators (Stage 1) and
rankers (Stage 2). Anonymisation via shuffled labels (A, B, C…) reduces but does not
eliminate self-identification bias — models may recognise their own writing style.

The synthesis covers positional bias and authority bias but not this specific risk.

**Design decision required:** Use separate ranker models, or accept self-evaluation with
anonymisation as sufficient mitigation?

### 1.3 Council type: model set, strategy, or both?

The synthesis describes multiple strategies (Voting, Generate→Rank→Refine, Debate, MoA,
Peer Review) as equally available. In practice, adding a new strategy requires either
redesigning the core interface or adding a dispatch layer. The two dimensions:

1. **Model set** — same algorithm, different models ("expert council" vs. "fast council").
   Cheapest; no interface change needed.
2. **Strategy** — different deliberation algorithm per type. Requires a strategy abstraction
   or dispatcher.
3. **Both** — a full registry combining model set and strategy per named council type.

**Design decision required:** Which scope for "first stage" — model sets only, or strategy
variants too?

### 1.4 LCCP first-stage scope

The synthesis presents a 12-state LCCP state machine with 4 configuration profiles and 4
conformance levels. For a first build, the implementation scope must be scoped explicitly.

If the first stage is "Karpathy peer review only, single round, no refinement," the state
machine reduces to:
```
INIT → GENERATE → VALIDATE_GENERATION → EVALUATE → VALIDATE_EVALUATION
     → AGGREGATE → DECIDE → FINALIZE → TERMINATE  (no REFINE loop)
```

Key scoping questions:
- Is a REFINE loop in scope for the first build, or deferred?
- Which conformance level is the target: Core, Safe, Robust, or Auditable?
- Are BestSoFar tracking and fallback finalization modes required from the start?

**Design decision required:** Explicitly state which LCCP states, invariants, and
conformance level are in scope for v1. This decision drives the entire architecture.

### 1.5 Quorum enforcement

If some council models fail to respond, the deliberation degrades silently. For the
peer-review design, if only one model responds, Stage 2 ranking is meaningless (a model
ranking its own answer). Kendall's W returns 0 for fewer than 2 rankers, but without a
quorum gate, Stage 3 proceeds anyway.

The synthesis defines `M_min` (minimum quorum) as a hard requirement. The new
implementation should enforce it explicitly with a clear error response rather than
producing a degenerate result.

**Design decision required:** What is `M_min` per council type? What is the response when
quorum is not met — error, or fallback to single-model answer?

### 1.6 Metadata persistence per message

The `labelToModel` mapping, aggregate rankings, and consensus score are produced per
council run. If not stored with the conversation message, audit and replay are impossible.
The synthesis recommends full traceability (Invariant I4).

**Design decision required:** Store full metadata with every assistant message, or treat
it as ephemeral? If stored, what schema?

### 1.7 Structured output vs. free-text parsing for rankings

In archive/v1, Stage 2 rankings were extracted from free-text model output via regex.
The failure mode (model deviates from expected format) silently produces an empty ranking,
which receives a mid-rank imputation in Kendall's W — degrading the consensus score with
no warning.

The synthesis recommends structured JSON output as best practice.

**Design decision required:** Require JSON output (via `response_format: json_object` on
OpenRouter) for ranking responses, with explicit failure handling when parsing fails?

---

## 2. Under-specified recommendations in the synthesis

### 2.1 Convergence criteria: which metric for which strategy?

§5.3 presents cosine embedding similarity as the primary convergence mechanism. But the
synthesis already covers Kendall's W (for rank-order strategies) and mentions LLM-as-Judge
as an option. The synthesis does not map metric to strategy:

- **Rank-order strategies** (peer review): Kendall's W is sufficient, no embedding calls.
- **Free-text strategies** (Debate, MoA): cosine similarity requires embedding
  infrastructure; LLM-as-Judge requires no extra dependencies.

The remaining unresolved question is not which metric to use in general — the synthesis
already addresses that — but which metric to use per strategy variant, and whether
LLM-as-Judge should be the v1 default for strategies without a natural rank ordering.

### 2.2 "Preserve unique insights" is not operationalised

The synthesis repeatedly recommends "explicitly prompt synthesis to preserve unique
insights." But neither the synthesis nor the Chairman prompt in archive/v1 defines what
makes an insight "unique" or how to distinguish a fringe claim from a minority-but-correct
insight. This is stated as a mitigation but not specified.

### 2.3 Trust weighting has no acquisition path

§5.1 and §9.4 reference trust-weighted scoring. The synthesis does not describe how trust
values are initialised, calibrated, or updated. Without a ground-truth oracle, trust
weights cannot be meaningfully set. For a first-stage implementation with no evaluation
framework, this feature should be deferred or treated as uniform weights.

### 2.4 "Council type" and "strategy" are conflated

§2 describes strategies. §11.5 describes a council type registry with model IDs and
strategy per type. The synthesis uses the terms interchangeably. In the new implementation
these should be distinct concepts: a **strategy** is a deliberation algorithm; a **council
type** is a named configuration combining a strategy with a model set and parameters.

### 2.5 The Chairman prompt receives untrusted Stage 2 input — internal contradiction

The synthesis contradicts itself on this point:

- §2.5 (Karpathy design): "Chairman receives all responses + all reviews" — raw Stage 2.
- §7 (Prompt Injection Propagation hazard): "pass only arbiter's summary, not raw output."
- §9.9 (Security): "Never pass raw responses; pass arbiter's validated summary."

The Karpathy design passes raw Stage 2 output; the safety recommendations say not to. A
decision is needed: sanitise/structure Stage 2 input to Stage 3, or accept the risk with
prompt-level mitigations?

---

## 3. Unclear design questions

### 3.1 Council type selection scope in the API

Given that context is stateless per query, council type selection is per-request. The
remaining question is the API shape:
- Field in the POST request body (most explicit)
- Query parameter (simpler, less RESTful)
- Server-wide config only (archive/v1 approach; no selection at runtime)

### 3.2 Should Stage 3 support graceful non-synthesis?

When council answers are strongly contradictory (consensus near 0), the Chairman should be
able to surface "the council could not reach a synthesis" as a first-class outcome rather
than being forced to synthesise anyway. The LCCP fallback chain — synthesize_top_k →
select_best → fallback_best_so_far — is not in the archive/v1 implementation and needs an
explicit decision.

### 3.3 Streaming architecture: stage events vs. token streaming

Archive/v1 streams stage-completion events over SSE (stage1_start, stage1_complete,
stage2_start, …). Token-by-token streaming within a stage would require passing a
streaming callback through the council pipeline. The design choice affects the core LLM
client interface.

---

## 4. Production constraints to design for from the start

### 4.1 JSON file storage has known limits

A `List()` that reads every conversation file is O(n) on disk. With many long conversations
storing full Stage 1/2/3 responses per message, this will get slow. The storage abstraction
(a `Storer` interface over a pluggable backend) should be designed from the start to allow
replacing the JSON backend later without rewriting handlers.

### 4.2 OpenRouter model lifecycle

Models on OpenRouter are deprecated, renamed, and temporarily unavailable. A council type
registry that stores specific model IDs by string will break when those IDs change. The
new implementation needs a model configuration layer that can be updated without redeploying,
separate from the council logic.

### 4.3 The `CalculateAggregateRankings` placement

In archive/v1, `CalculateAggregateRankings` lived on the `Runner` interface — a pure
computation function on a behaviour interface. Any new strategy implementation would also
need to satisfy it regardless of whether it produces rankings. In the new design this
should be a package-level function (or belong to a separate ranking/metrics layer), not
part of the deliberation interface.

### 4.4 Load-bearing infrastructure tasks before extending the API surface

The following were pending in archive/v1 and remain relevant for any new build:
- Config validation at startup (fail fast on missing API key)
- HTTP client timeout on the OpenRouter client
- Graceful shutdown for the HTTP server
- Structured logging (`slog`)
- Handler tests using mock interfaces

These are not optional niceties — they are prerequisites for a production REST API.
