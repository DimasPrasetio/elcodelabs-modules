# Source Priority

When two sources disagree, the higher one wins — but never resolve a real conflict silently.
Raise it as a finding instead.

1. `knowledge/decisions/ADR-*` — ratified decisions
2. This monorepo standard + `.claude/rules/`
3. Code, config, active migrations
4. `knowledge/*`
5. AI observations & hypotheses

Code shows what **actually happens**, not what **should**.
If (3) and (4) disagree, that's a conflict to report — not a choice to make on your own.
