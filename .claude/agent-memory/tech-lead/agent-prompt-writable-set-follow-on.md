---
name: agent-prompt-writable-set-follow-on
description: Widening an agent prompt's routing/target table must also widen its ABSOLUTE CONSTRAINTS writable set and its git-add line, or the agent is told to do what it is forbidden to do
type: project
---

When a `.claude/agents/*.md` change adds new **targets** to a routing/trigger table, three
things move together — they are part of the change, not polish:

1. The routing table row (the new target).
2. The agent's **ABSOLUTE CONSTRAINTS** writable-set clause (what it is allowed to touch).
3. The **Commit** step's `git add` line (what actually gets staged).

A target present in (1) but absent from (2) is a direct contradiction inside one prompt;
a target present in (1)+(2) but absent from (3) produces edits that are made and then
silently left uncommitted.

**Why:** observed on PR #352 (issue #351, reviewed 2026-09-08). The PR correctly replaced
`docs-maintainer.md`'s dead doc paths, and in doing so introduced `docs/openapi.yaml`
(not a `.md` file) and `README.md` (a `.md` outside `docs/`) as routing targets — while
constraint 1 still read *"Only `.md` files in `docs/`, `CLAUDE.md`, or `.proposals.md`"*
and step 5 still read `git add docs/ CLAUDE.md .proposals.md`. The pre-PR table happened
to target only `docs/*.md`, so the constraint had never been exercised. `docs-maintainer`
runs on haiku; a flat contradiction in its own prompt gets resolved arbitrarily.

Related trap in the same PR: `docs/openapi.yaml` is asserted by `internal/api/spec_test.go`.
Any agent authorized to edit it needs an explicit "run `go test ./internal/api/...` before
committing" instruction, or a doc-only agent can turn the Go suite red.

**How to apply:** at code review of any agent-prompt diff that touches a target/routing
table, grep the same file for its constraints clause and its `git add` line and require
all three to agree. Same mechanical-parity test as [[review-criteria-need-output-slot]],
one artifact over: there, criterion ↔ output slot; here, target ↔ permission ↔ staging.
Doc-routing content itself is governed by [[docs-triad-sync-gate]].
