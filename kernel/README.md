# Kernel 5 (pinned)

The kernel every module in this library builds on, pinned from monorepo-elcodelabs monorepo-elcodelabs/kernel/v5.0.
**Read-only**: nothing here is edited by hand — `npm run check` refuses any change that
kernel.lock.json does not record, and a kernel update arrives only through
`scripts/kernel-sync.mjs`, in a commit of its own.

- `host/` — a project scaffolded at this kernel: what modules install into. The testbed and
  `npm run sandbox` start from a copy of it. The kernel itself is
  `host/apps/api/internal/kernel/` and `host/apps/api/internal/shared/` (Go doc comments are
  its API reference), and `host/apps/web/src/shared/` for the web.
- `reference/` — the kernel contract and architecture, as the skill states them:
  `backend-go.md` (kernel.Deps, module contract, kernel versions), `backend-modular-monolith.md`
  (modules never import each other), `frontend.md` (FrontendModule, slots, tokens),
  `api-standard.md` (/api/v1 conventions). Paths there are the project's (`apps/api/…`).
- `kernel.lock.json` — where this came from and every file's hash.

Need something the kernel does not offer? Build it inside the module, or behind a port. If it
cannot live in a module folder and many modules need it, open a "Kernel request" issue.
