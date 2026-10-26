# Rule: Database

Applies to: `apps/api/**`.

- A repository accesses only the tables its own module owns.
- Cross-module boundary rules (no joins, no foreign keys, relations as primitive IDs) are
  authoritative in `backend-modular-monolith.md` — not restated here.
- Migrations are forward-only and **expand/contract**: a release only adds (tables, nullable or
  defaulted columns, indexes), so the previous version still runs on the new schema — a
  rollback deploys the older image without undoing migrations. A column is dropped or renamed
  in a later release, once no deployed version reads it. Never edit a migration that ran
  anywhere; add a new one.
- One schema change per migration file: MySQL commits DDL at once, so a file that fails
  half-way cannot roll back.
- Times are stored in UTC (the API's DB sessions run in UTC); "today" for a user is computed
  in Asia/Jakarta in code.
- Go: golang-migrate for migrations (timestamp-versioned; CI refuses one older than the newest
  on main — `node scripts/check-migrations.mjs <base> --fix` renames it), sqlc for access (one
  generated package per module), database/sql + MySQL driver. Not GORM.
