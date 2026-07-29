# PicHub-Aggregator Project Rules

Start every session by reading `devdocs/ARCHITECTURE.md` for project context.

## Key Conventions
- **Go backend** (Gin + SQLite with CGO) + **Vue3/TS/Tailwind frontend**
- **Default port**: 5721
- **AdminAuth** only protects POST/PUT/DELETE; all GET endpoints are public
- `RecordStats` runs async (`go`), does not block request path
- Do NOT push commits unless explicitly asked

## Git
- No force push, no `-i` interactive, no empty commits
- Before committing, inspect `git status`, `git diff`, `git log --oneline -10`
- Write concise commit messages matching repo style

## Images
- GHCR: `ghcr.io/untitled572/pichub-aggregator`
- ACR: `crpi-1ml0uqt093yc1v96.cn-beijing.personal.cr.aliyuncs.com/untitled572/pichub`

## Docs
- `devdocs/ARCHITECTURE.md` — architecture overview
- `devdocs/IMAGE_FLOW.md` — request lifecycle, source selection, URL extraction
- `docs/API_DOC.md` — API endpoints (user-facing)
