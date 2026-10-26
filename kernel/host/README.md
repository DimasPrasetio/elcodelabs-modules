# Testbed App

Monorepo generated to Dimas' standard (modular-monolith backend + React 19 frontend).

## Development

Run frontend and backend separately from the project root:

```bash
npm run dev:web
npm run dev:api
```

## Structure

- `apps/web` — React 19 + Tailwind 4 frontend
- `apps/api` — go modular-monolith backend
- `packages/` — shared code and API contract
- `knowledge/` — the project's one knowledge base; read the relevant file before editing,
  starting at `knowledge/INDEX.md`
- `.claude/rules/` — path-scoped technical rules
- `analysis/` — per-requirement analysis artifacts (see `analysis/README.md`)
- `infra/` — Docker image, host nginx site, and the elcodelabs VPS deploy files
- `.github/workflows/` — `ci` (checks) and `release-image` (builds the image to GHCR)

## Deployment

One image, one container on the elcodelabs VPS: built in CI, pulled by the server, database on
the host — see `knowledge/DEPLOYMENT.md`.
