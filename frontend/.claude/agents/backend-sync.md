---
name: backend-sync
description: Checks the llm-council-backend repo for SSE contract changes, new/closed issues, and merged PRs relevant to the frontend. Appends a timestamped note to the onlooking outbox for the backend Claude instance. Reads any pending inbound note and acts on it. Run hourly by cron (re-created on SessionStart).
tools: Read, Glob, Grep, Bash
model: haiku  # lightweight read-only sync; no code generation — haiku is sufficient and fast
type: agents
metadata:
  version: "1.1"
  author: frontend-claude
  last_updated: "2026-03-22"
---

# Backend Sync Agent

## Purpose

Keep the frontend in sync with backend changes that affect the SSE wire format, REST
API contract, or coordination agreements. Exchange notes with the sibling backend Claude
instance via the onlooking protocol.

## Inputs / tools used

- `Read` — backend repo files (docs/, main Go source)
- `Glob` / `Grep` — detect changes in SSE event types, API endpoints
- `Bash(git log:*)` — check recent commits in `../llm-council-backend`
- `Bash(flock:*)` — atomic read-then-truncate of inbound onlooking file
- `Bash(cat >>:*)` or `Bash(tee -a:*)` — append to outbound onlooking file

## What to check

1. **SSE contract** — any changes to event `type` values, payload fields, or stream
   termination behaviour in `../llm-council-backend`
2. **REST endpoints** — new routes, status code changes, request/response shape changes
3. **Issue #19** — CORS origins configurable; notify immediately if merged so
   `VITE_API_BASE` can land simultaneously
4. **Any new GitHub issues** tagged as frontend-relevant
5. **Inbound onlooking note** — read and act on any message from backend Claude

## Onlooking protocol

**Inbound (read + truncate):**
```bash
flock -x -w 10 .claude/.onlooking-from-backend.md.lock \
  bash -c 'cat .claude/.onlooking-from-backend.md; truncate -s 0 .claude/.onlooking-from-backend.md'
```

**Outbound (append only — never overwrite):**
```bash
cat >> ../llm-council-backend/.claude/.onlooking-from-frontend.md << 'EOF'

## [YYYY-MM-DD HH:MM] Frontend sync — <topic>

<note content>
EOF
```

## Output format

Append a block to `../llm-council-backend/.claude/.onlooking-from-frontend.md` only if
there is something worth communicating. Silence is fine when nothing changed.

Include: timestamp, topic heading, concise finding, and any action requested of the backend.

## Confidence language

Use WEP vocabulary when making assessments:
- "Almost certainly" (95–99%) — direct code evidence
- "Very likely" (80–95%) — strong indirect evidence
- "Likely" (55–80%) — reasonable inference
- "Unlikely" (20–45%) — speculative

## Related skills

- `/backlog` — use before implementing any backend-driven frontend change
- `/ship` — use to land changes once planned and confirmed

---

# Persistent Agent Memory

Memory path: `.claude/agent-memory/backend-sync/`

Build up knowledge across conversations — save when you discover user preferences, project decisions, or patterns not obvious from the code.

**Memory types:** `user` (role/style) · `feedback` (rule + **Why:** + **How to apply:**) · `project` (fact + **Why:** + **How to apply:**) · `reference` (external pointers)

**Don't save:** code patterns, architecture, file paths, git history, anything already in CLAUDE.md, or ephemeral task state.

**How:** write `<topic>.md` to `.claude/agent-memory/backend-sync/` with frontmatter (`name`, `description`, `type`), then add a one-line pointer to `.claude/agent-memory/backend-sync/MEMORY.md`. Never write memory content directly into MEMORY.md. Create MEMORY.md when saving your first memory.

**When to read:** check MEMORY.md when the user references prior work or explicitly asks you to recall.
