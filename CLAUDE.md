# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

LLM Council — a 3-stage multi-LLM deliberation system. Council models independently answer a query, anonymously peer-review each other, and a Chairman model synthesizes a final answer.

See `docs/` for full documentation:
- `docs/architecture.md` — system overview, components, data flow
- `docs/council-stages.md` — detailed stage logic and anonymization
- `docs/go-implementation.md` — Go package structure and implementation notes

## Stack

- **Backend:** Go (replacing original Python/FastAPI)
- **Frontend:** React + Vite (unchanged)
- **LLM Gateway:** OpenRouter API
- **Storage:** JSON files in `data/conversations/`

## Development

```bash
# Backend
go run ./cmd/server

# Frontend
cd frontend && npm run dev
```

## Notes

- Run backend from project root (not from `cmd/server/`)
- API key in `.env`: `OPENROUTER_API_KEY=sk-or-v1-...`
- Backend port: 8001 (frontend dev proxy points to this)
- Stage 2 `labelToModel` mapping is ephemeral — not persisted, only returned in API response
