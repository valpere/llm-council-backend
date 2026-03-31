---
name: Frontend security posture (PR #52)
description: Security architecture decisions and known issues introduced when the React frontend was merged into the monorepo
type: project
---

Frontend merged into monorepo in PR #52 (2026-03-31).

**Positive controls in place:**
- All LLM output rendered through react-markdown — no raw HTML injection found.
- No hardcoded secrets or API keys in JS source.
- No dynamic code execution patterns found.
- No unvalidated redirects found.
- api.js is the sole HTTP/fetch boundary — components do not call fetch directly.
- CORS is allowlist-based on the Go backend (localhost:5173, localhost:3000).
- VITE_API_BASE is a build-time env var, not a runtime injection point.

**Fixed in PR #52:**
- ReDoS in Stage2.jsx deAnonymizeText: `new RegExp(label, 'g')` replaced with `split(label).join(...)` — immune to regex metacharacters.

**Known open issues (GitHub issues):**
- #53: No CSP / security headers on Go backend responses. Severity: LOW.
- #54: Backend error strings surfaced in Stage3 UI — information disclosure if internal errors leak. Severity: LOW.

**Why:** Identified during PR #52 security review.

**How to apply:** When reviewing frontend/src/components/Stage2.jsx, the deAnonymizeText function no longer uses RegExp — confirm split/join pattern is preserved. For Go API changes, check if security headers (#53) have been added.
