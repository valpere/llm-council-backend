---
name: find-bugs
description: Find bugs, security vulnerabilities, and code quality issues in local branch changes. Use when asked to review changes, find bugs, security review, or audit code on the current branch. Report-only — no code changes.
user-invocable: true
argument-hint: "[topic] — optional focus area (security, correctness, etc.)"
metadata:
  version: "1.0"
  author: frontend-claude
  last_updated: "2026-03-21"
---

# /find-bugs

Review changes on this branch for bugs, security vulnerabilities, and code quality issues.
Report only — do not make changes.

## Phase 1: Complete Input Gathering

1. Get the full diff: `git diff $(git merge-base main HEAD)...HEAD`
2. If output is truncated, read each changed file individually until every changed line is seen
3. List all files modified in this branch before proceeding

## Phase 2: Attack Surface Mapping

For each changed file, identify and list:

- All user inputs (form fields, URL params, query strings)
- All HTTP requests and SSE stream handling
- All authentication/session state operations
- All external calls (`fetch`, `EventSource`)
- All state mutations that affect rendering

## Phase 3: Security Checklist (check EVERY item for EVERY file)

- [ ] **XSS**: All user-supplied content rendered via JSX (not `dangerouslySetInnerHTML`)? Any raw HTML injection?
- [ ] **Injection**: Any user input passed to `eval`, `innerHTML`, template literals in dangerous contexts?
- [ ] **CSRF**: State-changing requests use POST (not GET)?
- [ ] **Race conditions**: TOCTOU in read-then-write patterns? Concurrent state updates?
- [ ] **Information disclosure**: Error messages leaking internal details? Sensitive data in console logs?
- [ ] **Input validation**: Inputs sanitised before use? Numeric bounds checked?
- [ ] **DoS**: Unbounded loops triggered by user data? Missing error handling on SSE stream failures?
- [ ] **Business logic**: Edge cases in stage progression? State machine violations (e.g., stage3 rendered before stage1)?
- [ ] **SSE contract**: Event handlers covering all event types including `error`? Stream close handled?

## Phase 4: Verification

For each potential issue:

- Check if it's already handled elsewhere in the changed code
- Read surrounding context to confirm the issue is real (not already guarded)
- Note whether a browser or Node environment affects the risk

## Phase 5: Pre-Conclusion Audit

Before finalizing:

1. List every file reviewed and confirm it was read completely
2. List every checklist item and note whether you found issues or confirmed clean
3. List any areas you could NOT fully verify and why
4. Only then provide final findings

## Output Format

**Prioritize:** security vulnerabilities > bugs > code quality

**Skip:** stylistic/formatting issues

For each issue:

- **File:Line** — brief description
- **Severity:** Critical / High / Medium / Low
- **Problem:** what's wrong
- **Evidence:** why this is real (not already fixed, no existing guard)
- **Fix:** concrete suggestion

If you find nothing significant, say so — don't invent issues.

Do not make changes — just report findings.
