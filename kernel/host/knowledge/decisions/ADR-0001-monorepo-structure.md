# ADR-0001: Monorepo structure

## Status
Accepted

## Context
Testbed App uses a single repository for the frontend (`apps/web`) and backend (`apps/api`)
plus shared packages.

## Decision
Adopt the Dimas monorepo standard: apps/, packages/, knowledge/, .claude/rules/, analysis/, infra/.
`knowledge/` is the single knowledge rack — there is no separate `docs/` for stakeholders.
Enforced while editing by `.claude/rules/monorepo.md`.

## Consequences
Consistent structure and one home per fact. The cost is discipline: a new document has to
claim an existing owner's job or justify a new one, rather than being added alongside.
