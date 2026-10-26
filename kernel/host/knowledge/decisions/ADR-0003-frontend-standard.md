# ADR-0003: Frontend standard

## Status
Accepted

## Context
We want a consistent, simple frontend stack.

## Decision
Adopt the standard React 19 + Tailwind 4 stack. The stack list and its conventions are
authoritative in `.claude/rules/frontend-react.md`; this ADR records only why it was chosen:
one obvious way to do routing, state, validation, and HTTP, with nothing added speculatively.

## Consequences
Predictable structure and simple data patterns. No server-cache layer (e.g. TanStack Query)
until a real need proves one — revisit this ADR rather than adding it silently.
