# Testbed App — Claude Code Gateway

Project: Testbed App. Backend: go (modular monolith). Frontend: React 19 + Tailwind 4.

## Commands (run from root)

```bash
npm run dev:web   # start frontend
npm run dev:api   # start backend
```

## Critical architecture rules

These few invariants are restated here on purpose: this file is always loaded and
`.claude/rules/` is not. `.claude/rules/` holds the authoritative, path-scoped version — if
the two ever disagree, the rules file wins and the drift is a finding to report.

- Backend is a **modular monolith**. A module never imports another module — not even its
  `contracts/`. It declares what it needs as ports in its own `ports/`; modules are connected
  only in the composition root. No cross-module DB joins or foreign keys.
- Cross-module relations are stored as primitive IDs and resolved through the module's ports.
- Frontend uses the `@/*` alias, lazy routes, Zustand, Zod, Axios. UI is built from shadcn/ui
  primitives and semantic theme tokens — never raw colors.
- A module is wired in only through its registry, between the `elco:*` markers.
- Deploy = one image, one container on the elcodelabs VPS, built in CI and only pulled by the
  server; the database is the host's shared MySQL. Rules: `.claude/rules/deployment.md`.

## Before changing code

Read the relevant file in `knowledge/` first (start at `knowledge/INDEX.md`), and follow
the path-scoped rules in `.claude/rules/`. Beyond the invariants above, do not copy their
content into this file — route to it. This is a gateway, not a documentation dump.

If two sources of truth disagree, follow `knowledge/SOURCE_PRIORITY.md` — and report the
conflict, don't silently pick one.
