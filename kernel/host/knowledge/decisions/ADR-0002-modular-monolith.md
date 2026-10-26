# ADR-0002: Modular monolith backend

## Status
Accepted

## Context
We want clean module boundaries without the operational cost of microservices.

## Decision
Backend is a modular monolith whose modules never import each other: each declares what it
needs as ports and offers contracts, connected only in the composition root. The enforceable form of
this decision — what exactly is forbidden — is authoritative in
`.claude/rules/backend-modular-monolith.md`; this ADR records only why it was chosen.

## Consequences
Modules stay decoupled and could be extracted later if ever needed. The price paid on purpose:
cross-module reads go through a client instead of a join, so some reads cost an extra hop.
