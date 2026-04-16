---
name: ship
description: Full issue lifecycle — pick an issue, branch, implement, PR, Copilot review (one round), merge, close issue, return to main. One issue at a time.
user-invocable: true
argument-hint: "[issue-number|issue-title] — omit to pick from the top 5 open issues"
metadata:
  version: "2.1"
  author: frontend-claude
  last_updated: "2026-03-22"
---

# /ship

## Rules

- One issue at a time. Never work on two issues in parallel.
- Only ship PRs created by Claude or explicitly named by the user. Never touch Dependabot or third-party PRs.
- Branch protection is on — no direct pushes to `main`. Always go through a PR.
- One round of Copilot comments only. After fixing, do not wait for re-review.
- If Copilot has no comments (or only approved), merge immediately.
- Always return to `main` and pull after merge before picking the next issue.

## Steps

### 1. Pick issue

Three entry points:

- **`/ship <number>`** — use that issue number directly.
- **`/ship <title>`** — run `gh issue list --state open --search "<title>"`, pick the best match, confirm with the user.
- **`/ship` (no argument)** — list the top 5 open, **unblocked** issues by priority:

```
Open issues (top 5):

  1. [p3] gh#12  fix(api)      — Buffer partial SSE lines across chunks
  2. [p3] gh#14  chore(dx)     — Add ASOL frontmatter to backend-sync agent
  3. [p3] gh#15  refactor(app) — SSE event switch → named handler map

Select 1–3:
```

Exclude issues labelled `blocked` or noted as blocked in their body. If every open
issue is blocked, say "All open issues are currently blocked" and stop — no selection.

Wait for the user to select. Do not proceed until confirmed.

### 2. Branch

```bash
git checkout main && git pull
git checkout -b <type>/<short-description>
```

Branch naming: `feat/…`, `fix/…`, `docs/…`, `refactor/…`, `chore/…`

### 3. Implement

Read the issue. Make all necessary changes. Run lint after:

```bash
npm run lint   # must pass before continuing
```

Commit with conventional format: `fix(scope): description` / `feat(scope): description`

### 4. Pre-PR review (parallel)

Before creating the PR, launch **security-reviewer** and **static-analysis** simultaneously in a single Agent tool call batch. Wait for both to complete.

- **security-reviewer**: checks for XSS risks, injection, hardcoded secrets, insecure patterns
- **static-analysis**: verifies lint passes and flags any cosmetic violations missed

Address any CRITICAL or HIGH security findings and any remaining lint violations before continuing. LOW/MEDIUM security findings: note in the PR description.

### 5. Pre-flight

```bash
git status                         # nothing uncommitted
git log main..HEAD --oneline       # commits look right
```

### 6. Create PR

Include `Closes #N` in the body so GitHub closes the issue on merge.

```bash
git push -u origin <branch>
gh pr create --title "<debt-emoji> <type>(<scope>): <title>" --body "$(cat <<'EOF'
## Summary
<bullet points from commits>

Closes #N

## Test plan
- [ ] Dev server starts (`npm run dev`)
- [ ] Feature works end-to-end with backend running
- [ ] No console errors

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Debt emoji in title: `⚡` quick-fix · `⚖️` balanced · `🏗️` proper-refactor

### 7. Wait for Copilot — check yourself

```bash
gh pr view <number> --json reviews,statusCheckRollup
```

Poll every ~30s (timeout 5 min). Do not ask the user — check the status yourself.
If Copilot doesn't review within 5 minutes, proceed to merge anyway.

### 8. Address comments (if any)

One round only. Use the Code Review Pyramid to prioritise:
- Layer 1 (Architecture) → Layer 2 (Correctness) → Layer 3 (Documentation)
- Skip Layer 5 (Style) — automated by ESLint

Fix all actionable Copilot comments, push, then proceed to merge without waiting for re-review.

### 9. Merge

```bash
gh pr merge <number> --squash --delete-branch
```

### 10. Return to main

```bash
git checkout main && git pull
```

### 11. Report

Summarise: issue closed, PR number, what pre-PR review found (if anything), what Copilot flagged (if anything), what was fixed, merge commit.

## What NOT to do

- Do not bump version numbers or update changelogs unless explicitly asked.
- Do not open follow-up issues unless a Copilot comment reveals a real bug outside the PR scope.
- Do not ask the user to check Copilot — check it yourself.
- Do not work on the next issue until the current one is merged and main is clean.
