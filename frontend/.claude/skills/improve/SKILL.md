---
name: improve
description: Critique any plan, architecture decision, feature design, or implementation approach. Researches best practices, identifies flaws and gaps, and gives a SHIP IT / IMPROVE IT / RETHINK IT / KILL IT verdict with concrete fixes.
user-invocable: true
argument-hint: "<topic/description> — plan, proposal, or implementation to critique"
metadata:
  version: "1.0"
  author: frontend-claude
  last_updated: "2026-03-21"
---

# /improve

Technical improvement critic. Research-first — never start from zero.

```
Our idea → Find best references → Extract patterns → Critique → Improve → Ship
```

## When to run proactively

Suggest `/improve` when:
- A new architecture decision is being made
- A new feature is being designed before implementation
- A proposal in `.proposals.md` is being elevated to active work

Say: "This is new — want me to run /improve on it before we build?"

## Procedure

### Step 1: Understand the subject

- What is it? (architecture, feature, API design, state shape, pattern)
- What problem does it solve?
- What's the current proposal or draft?

### Step 2: Research

**Internal first:**
- Read `CLAUDE.md` for architecture constraints
- Read `.proposals.md` for related decisions
- Read `docs/api-contract.md` and `docs/streaming.md` if relevant to SSE or API design
- Read `docs/architecture.md` for system overview

**External if needed:**
| Domain | Sources |
|--------|---------|
| React / JSX | React docs, react.dev |
| State management | React docs (useState, useReducer) |
| SSE / streaming | MDN EventSource, Fetch streaming docs |
| Security | OWASP, MDN |
| JS performance | MDN, web.dev |

### Step 3: Structured critique

#### 3A. Architecture alignment
- Does this respect App.jsx as the single state owner?
- Does it follow the SSE adapter pattern (api.js boundary — components never see raw SSE)?
- Does it introduce coupling between components that should be independent?

#### 3B. Flaws & risks
- What can go wrong?
- What's the worst-case scenario (blank UI, silent failure, user confusion)?
- What assumptions could be wrong?
- What race conditions or edge cases exist?

#### 3C. Best-practice gap
- How does this compare to the references found?
- What good implementations have that ours is missing?
- What are we overcomplicating?

#### 3D. Simplicity check (YAGNI / KISS)
- Can this be simpler?
- What's the minimum viable version?
- What can be cut without losing core value?

#### 3E. Testability
- Can this be verified manually end-to-end?
- Are there observable signals when it works or fails?

#### 3F. Security
- Is any user-supplied data rendered without sanitisation?
- Could this expose information to unintended parties?

### Step 4: Improvement proposals

For each issue:

```
ISSUE: [what's wrong or missing]
REFERENCE: [who does it better and how]
FIX: [specific change to make]
IMPACT: [what improves if we do this]
EFFORT: Low / Medium / High
```

### Step 5: Verdict & score

| Dimension | Score (1–10) | Notes |
|-----------|-------------|-------|
| Architecture alignment | | |
| Completeness | | |
| Simplicity | | |
| Best-practice match | | |
| Security | | |
| **Overall** | | |

- **SHIP IT** (8+) — good enough, minor tweaks only
- **IMPROVE IT** (5–7) — solid foundation, needs specific fixes before building
- **RETHINK IT** (3–4) — core approach has issues, consider alternatives
- **KILL IT** (<3) — doesn't serve the goals, redirect energy elsewhere

### Step 6: Apply or propose

If SHIP IT or IMPROVE IT: apply fixes directly, update relevant files.

If RETHINK IT: present 2–3 alternative approaches with pros/cons and the references that inspired each.

If KILL IT: explain clearly why, suggest where energy should go instead.

## After a significant decision

If a significant architectural decision was made, update `.proposals.md`:
- Add to Active (if proposing) or move to Done (if decided and implemented)
- Note the decision and reasoning concisely
