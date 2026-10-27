# Plans

A module that owns tables, needs ports, or carries business rules is designed here before it
is built: `PLAN-<NAME>.md`. The maintainer approves the plan's decisions; the plan then records
what changed while building and the evidence (testbed, reviews). Plans are history — keep them
after release; the module's README is what users read.

A plan has, in this order:

1. **Goal and limits** — what it is for, and what it is not (and which module that belongs to).
2. **Decisions to approve** — a table: decision, recommendation, alternatives, with sources.
3. **Design** — tables (migration SQL), contract (Go interface), ports, permissions, pages.
4. **How other modules use it** — ports and adapters, never imports.
5. **Tests** — unit tests and the testbed scenario's checks.
6. **Risks to verify while building.**
7. **Order of work.**
8. After building: **Hasil pembangunan / Results** — what was built, deviations with reasons,
   review findings and their fixes, accepted limits.

| Plan | Modules | Status |
|---|---|---|
| [PLAN-MODUL-ORGANIZATION](PLAN-MODUL-ORGANIZATION.md) | `organization` | released 0.1.0 |
| [PLAN-FONDASI-DOKUMEN](PLAN-FONDASI-DOKUMEN.md) | `money`, `docstate`, `period`, `numbering` | released 0.1.0 |
| [PLAN-MODUL-PELANGGAN-PEMASOK](PLAN-MODUL-PELANGGAN-PEMASOK.md) | `customers`, `suppliers`, `taxid` | released 0.1.0 |
