# /self-learn Skill — Portability Guide

How to adopt this skill for any project.

---

## What this skill does

`/self-learn` is a structured feedback loop. It captures mistakes and wins as
structured JSONL, automatically promotes recurring patterns to hard rules in
CLAUDE.md, and runs periodic retrospectives to extract systemic insights.

The system gets smarter with every interaction. By week four, you'll have a
knowledge base that prevents 80% of repeat mistakes and surfaces proven
approaches automatically.

---

## Quick Setup (3 minutes)

### 1. Copy the skill

Copy this entire directory into your project:

```
.claude/skills/self-learn/
  SKILL.md                 # the skill definition
  PORTABILITY-GUIDE.md     # this file
```

### 2. Run init

```
/self-learn init
```

This creates the data directories if they don't exist:

```
_patterns/
  mistakes.jsonl
  wins.jsonl
  cross-project.md
  anti-patterns.md
_knowledge-base/
  decisions.md
  api-docs/
```

### 3. Add to .gitignore (recommended)

Pattern files contain project-specific learning data. Whether to commit them
is a team decision:

**Option A — Gitignore (solo developer or sensitive data):**
```gitignore
_patterns/mistakes.jsonl
_patterns/wins.jsonl
```

**Option B — Commit everything (team learning):**
Keep all files tracked. The team benefits from shared patterns.

**Option C — Commit summaries only:**
```gitignore
_patterns/mistakes.jsonl
_patterns/wins.jsonl
# Keep cross-project.md, anti-patterns.md, and decisions.md tracked
```

### 4. Start logging

After any significant task:
```
/self-learn log
```

Weekly or bi-weekly:
```
/self-learn retro
```

---

## Customization

### Project name

The skill auto-detects the project from the working directory name. If you
want a specific name, set it in `.claude/skills/self-learn/config` (create
the file):

```
project_name: My Project
```

### Starter knowledge

The `cross-project.md` file ships with generic patterns from API integration,
workflow design, and communication. Replace or extend these with your
domain-specific patterns:

- **Backend project** — add database migration patterns, ORM gotchas
- **Frontend project** — add state management patterns, CSS pitfalls
- **Automation project** — add platform-specific patterns (Make.com, n8n, Zapier)
- **API-heavy project** — add per-API gotcha files in `_knowledge-base/api-docs/`

### Hard rule section in CLAUDE.md

When patterns get promoted, they're added under a `## Self-Learning Hard Rules`
section in your project's CLAUDE.md. If this section doesn't exist, the skill
creates it automatically.

You can rename this section by editing the SKILL.md — search for
`Self-Learning Hard Rules` and replace with your preferred heading.

### Retrospective cadence

| Project activity | Recommended cadence |
|-----------------|-------------------|
| Active development (daily commits) | Weekly |
| Maintenance mode | Bi-weekly |
| Sprint-based | End of each sprint |
| Ad-hoc | When mistakes.jsonl hits 10+ entries |

---

## Integration with other skills

### /ship

After `/ship` completes (PR merged), consider logging:
- Win: if the pipeline ran smoothly
- Mistake: if /fix-review or the Arbiter caught something that should have been prevented earlier

### /fix-review

After `/fix-review` completes, the arbiter dismissals and confirmations are
good candidates for pattern logging:
- Recurring false positives → anti-pattern (for the model, not your code)
- Recurring real findings → mistake pattern to track

### /live-test

After a `/live-test` session, issues found are candidates for logging:
- Bugs that should have been caught earlier → mistake
- Test flows that caught real issues → win

---

## How the promotion cycle works

```
Day 1:  You make a mistake → logged to mistakes.jsonl
Day 5:  Same mistake again → promoted to CLAUDE.md hard rule
Day 10: Retro runs → grouped with related patterns, severity analyzed
Day 15: 3rd occurrence → promoted to anti-patterns.md (stop trying this)
```

The key insight: **log confirmations, not just corrections**. If you only
track mistakes, the system becomes overly cautious and drifts from validated
approaches. Tracking wins preserves what works.

---

## File formats

### mistakes.jsonl (one JSON object per line)

```json
{"date":"2026-03-30","project":"growth-core","task":"Steam API sync","mistake":"Used wrong date format (ISO instead of DD-MM-YYYY)","resolution":"Added format conversion in sync function","pattern":"Always check date format expectations before first API call","severity":"medium"}
```

### wins.jsonl (one JSON object per line)

```json
{"date":"2026-03-30","project":"growth-core","task":"Manager RPC design","win":"Used SECURITY DEFINER RPCs instead of client-side service key","pattern":"Server-side functions for cross-RLS access are safer than exposing service keys","reusable_in":"any Supabase project with role-based data access"}
```

### Severity guide

| Severity | When to use |
|----------|------------|
| **high** | Production impact, data loss, or >30 min rework |
| **medium** | Multiple retries, wrong assumptions that cascaded |
| **low** | Minor inconvenience, caught quickly |
