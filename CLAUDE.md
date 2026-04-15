# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

LLM Council — a multi-LLM deliberation system. Council models independently answer a query,
anonymously peer-review each other, and a Chairman model synthesizes a final answer.

**Status: planning phase.** The v1 implementation is archived on `archive/v1`. A rewrite
is in progress starting from the research documents below.

See `docs/` for the current source of truth:
- `docs/council-research-synthesis.md` — aggregated design research (strategies, LCCP state machine, Go patterns, production considerations)
- `docs/council-research-gaps.md` — design decisions and open questions to resolve before building

The following docs describe the **archived v1** implementation and are retained for reference only:
- `docs/architecture.md`, `docs/council-stages.md`, `docs/go-implementation.md`, `docs/user-guide.md`

## Stack (planned for v2)

- **Backend:** Go
- **LLM Gateway:** OpenRouter API
- **API key:** `.env` → `OPENROUTER_API_KEY=sk-or-v1-...`

## Workflow

Full pipeline:
```
/backlog → Tech Lead (APPROVED) → gh issue create → plan file deleted
    → /ship → code-generator → [/fix-review rounds] → squash merge
```

### Skills

| Skill | Invoke | Purpose |
|-------|--------|---------|
| `/backlog` | `/backlog <task or issue#>` | Plan → Tech Lead gate → creates GitHub issue → deletes plan file |
| `/ship` | `/ship` | Select issue → implement → PR → Copilot → `/fix-review` → squash merge |
| `/fix-review` | `/fix-review [pr#]` | 3-round review (security + simplifier + tech-lead) + arbiter |
| `/find-bugs` | `/find-bugs` | Audit current branch changes for bugs/security — report only |
| `/improve` | `/improve <target>` | Critic pass: SHIP IT / IMPROVE IT / RETHINK IT / KILL IT |

### Agents (invoked by skills)

| Agent | Model | Role |
|-------|-------|------|
| `tech-lead` | opus | Approves plans + reviews code; architectural authority |
| `code-generator` | sonnet | Implements Tech Lead-approved plans |
| `bug-fixer` | sonnet | Targeted bug fixes; one bug, one commit |
| `docs-maintainer` | sonnet | Post-merge doc sync only |
| `ci-build-agent` | sonnet | Generates GitHub Actions CI workflows for Go + npm |
| `pm-issue-writer` | sonnet | Drafts RFC 2119 GitHub issues with structured frontmatter |

### Plans

Implementation plans live in `.claude/plans/`. Naming: `{N}-{slug}.md` where N is the
priority digit (0=critical, 3=low). Each plan has frontmatter with type, priority,
labels, and `github_issue` filled after issue creation.

See `.claude/plans/README.md` for the full schema.

### Debt levels

| Symbol | Level | Tests | Docs |
|--------|-------|-------|------|
| ⚡ | quick-fix | Happy-path only | Inline comments |
| ⚖️ | balanced | Core paths | Update if public API changed |
| 🏗️ | proper-refactor | Full unit + integration | Full update |

### Labels (GitHub)

**Type:** `bug` · `feature` · `task` · `test` · `docs`
**Priority:** `p0: critical` · `p1: high` · `p2: medium` · `p3: low`
**Status:** `blocked` · `wontfix` · `duplicate`

### PR workflow

1. Branch → implement → `go build/vet/test` all pass
2. `/ship` → creates PR → waits for Copilot review
3. Address comments → `/fix-review` → squash merge → `git checkout main && git pull`
