# Plans

Implementation plans for the LLM Council frontend.

Plans are created by the `/backlog` skill. Each plan is promoted to a GitHub issue, then the plan file is deleted — the issue becomes the source of truth.

---

## Naming Convention

```
{N}-{slug}.md
```

`N` is the priority digit (0 = highest, 3 = lowest). Matches the `p{N}` priority label.

| Prefix | Priority | Meaning |
|--------|----------|---------|
| `0-` | p0-critical | Blocker — broken UX, data loss, security. Do this now. |
| `1-` | p1-high | Top of the queue — ships this sprint. |
| `2-` | p2-medium | Should do — not blocking anything. |
| `3-` | p3-low | Nice to have — backlog. |

Examples: `0-stage3-error-ui.md`, `2-vite-api-base.md`, `3-sse-chunk-buffer.md`

---

## Frontmatter Schema

Every plan file starts with this YAML frontmatter:

```yaml
---
title: "Short human-readable title"
type: bug           # bug | feature | task | chore
priority: p0-critical  # p0-critical | p1-high | p2-medium | p3-low
status: draft       # draft | ready | in-progress | done | blocked
debt: quick-fix     # quick-fix | balanced | proper-refactor  (⚡/⚖️/🏗️)
effort: s           # xs | s | m | l | xl
component:          # api | stage1 | stage2 | stage3 | ui | config | dx
  - stage3
  - ui
labels:             # used verbatim as GitHub issue labels
  - bug
  - p0-critical
  - stage3
  - ux
blocked_by: null    # plan slug or GitHub issue number, e.g. "gh#18"
github_issue: null  # filled in after creation, e.g. "#42"
created: YYYY-MM-DD
updated: YYYY-MM-DD
---
```

### Type

| Value | Use |
|-------|-----|
| `bug` | Something is broken or missing |
| `feature` | New user-visible functionality |
| `task` | Non-code work: docs, config, CI |
| `chore` | Maintenance: deps, refactoring, DX |

### Priority

| Value | GitHub label | Meaning |
|-------|-------------|---------|
| `p0-critical` | `p0-critical` | Drop everything |
| `p1-high` | `p1-high` | Next sprint goal |
| `p2-medium` | `p2-medium` | This quarter |
| `p3-low` | `p3-low` | Backlog / nice to have |

### Status lifecycle

```
draft → ready → in-progress → done
                    ↓
                 blocked
```

### Debt level

| Value | Emoji | Meaning |
|-------|-------|---------|
| `quick-fix` | ⚡ | Targeted, minimal, no refactor |
| `balanced` | ⚖️ | Sensible trade-offs, some cleanup OK |
| `proper-refactor` | 🏗️ | Full refactor, break things cleanly |

### Effort

| Value | Meaning |
|-------|---------|
| `xs` | < 30 min, trivial |
| `s` | 1–2 hours, one sitting |
| `m` | Half a day |
| `l` | Full day |
| `xl` | Multiple days, needs breakdown |

---

## Plan File Structure

```markdown
---
(frontmatter)
---

## Summary
1–3 sentences. Problem statement + why it matters.
This section becomes the GitHub issue description.

## Acceptance Criteria
- [ ] Specific, testable outcome
- [ ] Another outcome

## Implementation

### Files to change
- `src/...` — what changes and why

### Files to read (context only)
- `src/...` — why relevant

### Approach
Step-by-step notes. Call out decisions with more than one reasonable answer.

### Risks / Unknowns
Use WEP vocabulary: "Very likely...", "Unlikely..."

## Not in Scope
Explicit list of what this plan intentionally excludes.

## Commit Message
\`\`\`
fix(scope): description ⚡
\`\`\`

## After Implementing
- [ ] `npm run lint` passes
- [ ] Manual smoke test
- [ ] `/ship` to create PR and merge
- [ ] `docs-maintainer` if SSE/API/architecture changed
- [ ] Move plan status to `done`, fill `github_issue` if created
```

---

## GitHub Issue Creation

After the plan is confirmed, `/backlog` offers to create a GitHub issue:

```bash
gh issue create \
  --title "<type>(<component>): <title>" \
  --label "<comma-separated labels>" \
  --body "$(cat <<'EOF'
## Summary
...

## Acceptance Criteria
- [ ] ...
EOF
)"
```

Once the issue is created, **the plan file is deleted** — the GitHub issue becomes the
source of truth. Plan files are scratch pads, not long-lived documents.
