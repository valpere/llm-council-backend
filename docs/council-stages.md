> **v1 reference — archived.** This document describes the implementation on `archive/v1`.
> For the active v2 planning documents see [`council-research-synthesis.md`](council-research-synthesis.md) and [`council-research-gaps.md`](council-research-gaps.md).

# Council Stages

The deliberation pipeline runs three sequential stages. Within Stage 1 and Stage 2, model queries run concurrently.

## Stage 1: Individual Responses

Each council model receives only the user's original query. Models are unaware they are part of a multi-model evaluation.

**Input:** User query
**Process:** Parallel queries to all `COUNCIL_MODELS` via OpenRouter
**Output:** `[]StageOneResult{Model, Response}`

## Stage 2: Anonymized Peer Review

Responses from Stage 1 are presented to all council models anonymously. Each model is asked to rank the responses from best to worst.

**Input:** User query + Stage 1 results
**Process:**
1. Shuffle Stage 1 results, then assign labels `Response A`, `Response B`, ... to the shuffled order
2. Build ranking prompt requiring strict `FINAL RANKING:` output format
3. Query all council models concurrently with this prompt
4. Parse rankings from each response using regex
5. Compute aggregate rankings (average position per model)

**Output:** `[]StageTwoResult{Model, Ranking, ParsedRanking}` + `labelToModel` map + `aggregateRankings` + `consensusW float64`

### Ranking Prompt Format

Models are instructed to produce output ending with exactly:
```
FINAL RANKING:
1. Response C
2. Response A
3. Response B
4. Response D
```

This strict format enables reliable regex extraction without needing another LLM call to parse the output.

### Aggregate Rankings

For each model, all ranking positions assigned by peers are collected and averaged. Lower average = better perceived quality. This is displayed as "Street Cred" in the UI.

### Kendall's W Consensus Score

`CalculateAggregateRankings` returns both the aggregate rankings slice and a `float64` consensus score computed via Kendall's coefficient of concordance (W).

- **W = 1.0** — perfect agreement across all council rankers
- **W = 0.0** — no agreement (rankings are random)
- **W ≥ 0.7** — interpreted as strong agreement
- **0.4 ≤ W < 0.7** — interpreted as moderate agreement
- **W < 0.4** — interpreted as weak agreement

Partial rankings (models that did not rank all items) are handled by assigning the midrank of the unranked positions, keeping W within [0, 1]. The score is passed to Stage 3 and included in the `stage2_complete` SSE event and in the `Metadata.ConsensusW` field of the API response.

## Stage 3: Chairman Synthesis

A single designated model (the Chairman) receives full context — all Stage 1 responses with model attribution and all Stage 2 rankings — and synthesizes a comprehensive final answer.

**Input:** User query + Stage 1 results + Stage 2 results (de-anonymized) + `consensusW float64`
**Process:** Single query to `CHAIRMAN_MODEL`. The `consensusW` score is formatted into a human-readable "CONSENSUS SCORE (Kendall's W)" block appended to the chairman prompt, guiding the synthesis tone: high agreement → confident single answer; low agreement → present multiple perspectives.
**Output:** `StageThreeResult{Model, Response}`

## De-anonymization

The `labelToModel` map produced in Stage 2 is:
- **Not persisted** to storage
- **Returned** in the API response alongside stage results
- **Used by the frontend** to replace `Response A/B/C/D` labels with actual model names in the Stage 2 display

This preserves evaluation integrity while providing transparency to the user.
