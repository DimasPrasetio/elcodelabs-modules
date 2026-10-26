# Rule: Monorepo

Applies to: whole repository.

- Frontend and backend start separately (the commands live in `CLAUDE.md`). Do not introduce a
  combined `npm run dev` or `concurrently` unless explicitly requested.
- `CLAUDE.md` carries only the always-loaded invariants; detail belongs in `knowledge/`.
- `knowledge/` is the project's only knowledge rack. There is no parallel `docs/`.
- Every fact has one owner. Before writing a fact into a second file, check whether the file
  that already owns it should be updated instead — and if a summary really is needed, name the
  authoritative file next to it.
- Do not use by default unless explicitly requested: Microservices, Kubernetes, Turborepo complexity, Nx, Redis, Memcache, TanStack libraries, multiple frontend containers, separate backend/frontend production containers, Dockerized database, cross-module foreign keys, cross-module joins, GORM (or GORM AutoMigrate) for Go.
