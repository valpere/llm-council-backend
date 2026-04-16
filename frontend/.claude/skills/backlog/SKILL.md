---
name: backlog
description: Plan the implementation of a feature or fix before writing any code. With no argument, shows the top 5 backlog items and asks the user to pick. Reads relevant files, identifies all changes needed, writes a plan file to .claude/plans/, offers to create a GitHub issue (deduplicating against existing open issues), and removes the plan file after the work is done.
user-invocable: true
argument-hint: "[task description] — omit to pick from the backlog"
metadata:
  version: "2.2"
  author: frontend-claude
  last_updated: "2026-03-21"
---

# /backlog

## Purpose

Prevent wasted effort by aligning on approach before touching code. For trivial changes
(one-line fix, typo) skip this and just do it. For anything that touches more than one
file or requires design decisions, plan first.

## Plan file lifecycle

The plan file is a **scratch pad** — it exists only while writing the plan. Once a
GitHub issue is created, the plan file is deleted and the issue becomes the source of
truth. Put everything worth keeping (summary, AC, approach, risks) in the issue body.

---

## Entry: no argument vs. explicit task

### Called with no argument — pick from backlog

1. List `.claude/plans/`, exclude `README.md` and redirect stubs (single `>` blockquote on line 1).
2. Sort by filename (numeric prefix = priority order).
3. Read frontmatter of each to extract `title`, `type`, `priority`, `status`, `effort`, `github_issue`.
4. Skip `status: done` and `status: blocked` unless nothing else remains.
5. Present the top 5:

```
Backlog (top 5):

  1. [p0] 0-stage3-error-ui     bug,   s  — gh#9   Stage3 panel blank on SSE error
  2. [p2] 2-skills-autodiscovery chore, xs — gh#11  Skills not auto-discovered as /commands
  3. [p3] 3-adapter-comment      chore, xs — gh#13  Add Adapter pattern JSDoc to api.js
  4. [p3] 3-sse-chunk-buffer     bug,   s  — gh#12  SSE lines can split across chunks
  5. [p3] 3-structured-backend-sync task, xs — gh#14 Upgrade backend-sync agent frontmatter

Select 1–5, enter a plan filename, or describe a new task:
```

6. Wait for selection. Number/filename → load plan, go to **Step 3**. New description → go to **Step 1**.

### Called with an argument — build a new plan

Go to **Step 1** with the argument as the task description.

---

## Steps (new plan)

### 1. Orient

- Check `.claude/plans/` for an existing plan covering this task. If found, go to **Step 3**.
- Run `gh issue list --state open --search "<keywords>"` to check for a duplicate GitHub issue.
  If one exists, note its number and link it — do not create another.
- Read `CLAUDE.md` and docs if relevant.

### 2. Build plan

Read every file that will change. Produce:

- **Files to change:** explicit list with what changes in each.
- **Files to read (context):** context only.
- **Approach:** step-by-step notes. Call out decisions with more than one reasonable answer.
- **Risks / Unknowns:** WEP vocabulary.
- **Not in scope:** intentional exclusions.
- **Commit message:** suggested conventional commit.

Assign frontmatter:

| Field | Options |
|-------|---------|
| `type` | `bug` \| `feature` \| `task` \| `chore` |
| `priority` | `p0-critical` \| `p1-high` \| `p2-medium` \| `p3-low` |
| `status` | `draft` \| `ready` |
| `debt` | `quick-fix` \| `balanced` \| `proper-refactor` |
| `effort` | `xs` \| `s` \| `m` \| `l` \| `xl` |
| `component` | `api` \| `stage1` \| `stage2` \| `stage3` \| `ui` \| `config` \| `dx` |
| `labels` | type + priority + component + domain tags |
| `blocked_by` | plan slug, `gh#N`, or `null` |

**Tech Lead gate:** invoke `tech-lead` agent if plan touches `src/App.jsx` state model,
`src/api.js` SSE adapter, new streaming components, or new API calls. Skip for doc/CSS/cosmetic.

Write plan to `.claude/plans/{N}-{slug}.md`. Do not include Summary or Acceptance Criteria
in the plan file — those go in the GitHub issue only.

### 3. Review plan

Present the plan:

```
## Plan: <title>  <debt emoji>

**Type:** …  **Priority:** …  **Effort:** …  **Debt:** …
**Labels:** …  **Issue:** gh#N (or —)

---

**Files to change:**
- `src/…` — …

**Approach:**
1. …

**Risks:** …

**Not in scope:** …

**Commit message:** `…`

---
Plan: `.claude/plans/{N}-{slug}.md`
Create a GitHub issue from this plan?
```

### 4. Offer GitHub issue

**Before creating:** run `gh issue list --state open --search "<title keywords>"`.
If a matching open issue is found, display it and ask whether to link to it instead
of creating a new one. Only create if no duplicate exists.

If creating:

```bash
gh issue create \
  --title "<type>(<component>): <title>" \
  --label "<comma,separated,labels>" \
  --body "$(cat <<'EOF'
## Summary
<1–3 sentences: problem and why it matters>

## Acceptance Criteria
- [ ] <specific, testable outcome>

**Plan:** `.claude/plans/{N}-{slug}.md`
EOF
)"
```

Record the issue number. Then immediately remove the plan file:

```bash
rm .claude/plans/{N}-{slug}.md
```

The GitHub issue is now the source of truth. The plan file's job is done.

### 5. Wait for confirmation

Stop. Do not write any code until the user explicitly confirms.
Amendments → update plan file, present again.

### 6. Implement

Execute the plan exactly. After implementation:

- [ ] `npm run lint` passes
- [ ] Manual smoke test
- [ ] `/ship` — PR, Copilot review, one fix round, squash merge (`Closes #N` in PR body), checkout main
- [ ] `docs-maintainer` if SSE events, REST endpoints, or architecture changed

### 7. Close issue on merge

Include `Closes #N` in the PR body — GitHub closes the issue automatically on merge.
The plan file was already removed in step 4.
