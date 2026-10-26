# ADR-0004: Deploy to the elcodelabs VPS — image from CI, server only pulls

## Status
Accepted

## Context
Testbed App runs on the shared elcodelabs VPS beside other apps. The box is small (2 GB RAM) and
follows one fixed convention for every app, so any of them can be deployed, debugged, or rolled
back the same way.

## Decision
Follow that convention as-is: one image and one container for the whole monorepo, built in CI
and pushed to GHCR, pulled and run by the server, on the host's shared MySQL. The enforceable
form is authoritative in `.claude/rules/deployment.md`; this project's concrete server facts
and procedure live in `knowledge/DEPLOYMENT.md`. This ADR records only why.

## Consequences
Builds never compete with running apps for the server's memory, and every release is an
immutable image that can be redeployed to roll back. The price paid on purpose: a release needs
a green CI run and a registry pull — there is no "quick fix directly on the server".
