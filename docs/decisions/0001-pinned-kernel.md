# ADR 0001 — The library pins its kernel and needs nothing else

**Status:** accepted, 26 October 2026.

## Context

Module developers may work on this repository alone, without access to (or a copy of) the
monorepo-elcodelabs skill. Until now the testbed and CI scaffolded projects with the skill, and
the kernel contract was documented only in the skill's references. The architecture and the
kernel must stay out of a module developer's reach: they change only in the skill.

## Decision

1. **Pinned host.** `kernel/host/` is a project scaffolded by a released skill (tag
   `monorepo-elcodelabs/kernel/v<N>.<R>`, exported with `git archive` — never a working tree),
   with the skill's kernel references in `kernel/reference/` and every file's SHA-256 in
   `kernel/kernel.lock.json`. Only `scripts/kernel-sync.mjs` writes `kernel/`.
2. **Guards.** `npm run check` (pre-commit and CI) refuses an edit under `kernel/`
   (`pinned-kernel`), a module whose `requires_kernel` is above the pinned kernel
   (`kernel-pinned`), and a commit that mixes `kernel/` with anything else (`kernel-alone`).
   `.github/CODEOWNERS` puts `kernel/`, `scripts/`, CI, and the contract documents under the
   maintainer's review (effective with branch protection "Require review from Code Owners").
3. **Self-serve tools.** The testbed copies the pinned host (`--skill` remains for trying an
   unreleased kernel); `npm run sandbox -- <ids>` builds a runnable project with the modules
   named, their required providers, kits, and the preset.
4. **One way to change the kernel.** A "Kernel request" issue; the maintainer decides (it cannot
   live in a module folder, and several modules need it), releases in the skill after running
   this testbed against it, and syncs here in a kernel-only commit.
5. **Compatibility.** A kernel only adds; removal takes a deprecating kernel first, then a
   breaking one with upgrade notes (skill: `references/backend-go.md` › Kernel versions).

## Why — and how strong the grounds are

| Part | Grounded in | Strength |
|---|---|---|
| A plugin states the minimum host it needs | VS Code `engines.vscode`; Grafana `grafanaDependency` | strong |
| Plugin work runs on a pinned, released host — not the host's source | VS Code tests download a set version; Grafana CI runs `GRAFANA_VERSION` images; xcaddy `CADDY_VERSION` | strong |
| That host is kept **in** this repository | No plugin ecosystem does exactly this (they download it). Closest: Go vendoring — "all files used for a build are stored in a single file tree", checked against go.mod. Chosen because developers here may have this repository only | adaptation |
| Review-enforced ownership | GitHub CODEOWNERS + required code-owner review (free on public repositories) | strong |
| A sandbox for one module | Backstage `createDevApp`; `xcaddy run` | strong |
| The kernel is tested against its consumers before release | Fowler, *Consumer-Driven Contracts* | strong |
| Additive kernels; removal only through a new version | Kubernetes deprecation policy, rules #1 and #4a | strong (the one-kernel window is a judgment for this scale) |
| A minimal kernel | Microkernel pattern (Richards): the core holds only the minimum | medium |

Not taken, with the trigger that would bring it: testing several kernels at once (client
projects stuck on older kernels); a package registry or separate SDK repository (module
developers outside the team needing versioned distribution); downloading the host from the
skill's releases (the skill guaranteed public and this repository's history growing too large).

## Evidence

Clean-clone acceptance, 26 October 2026: a fresh clone with `HOME` pointed at an empty folder
(no `~/.claude/skills`), no `SKILL_DIR`, no skill clone:
- `npm run check` passed.
- `npm run sandbox -- auth settings files mailer` produced a project (users, the four kits, and the
  preset came along; every port wired). It built, `go vet` was clean, and its unit tests passed.
- The testbed passed all five scenarios against MySQL, 179 checks, including the guard checks.
- A one-byte edit under `kernel/` was refused by `npm run check` and by the pre-commit hook.

Sources:
[Kubernetes deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/) ·
[GitHub code owners](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners) ·
[GitHub protected branches](https://docs.github.com/en/enterprise-cloud@latest/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches) ·
[Go vendoring](https://go.dev/ref/mod#vendoring) ·
[VS Code extension manifest](https://code.visualstudio.com/api/references/extension-manifest) ·
[VS Code testing extensions](https://code.visualstudio.com/api/working-with-extensions/testing-extension) ·
[Grafana plugin E2E CI](https://grafana.com/developers/plugin-tools/e2e-test-a-plugin/ci) ·
[Grafana plugin.json](https://grafana.com/developers/plugin-tools/reference/plugin-json) ·
[xcaddy](https://github.com/caddyserver/xcaddy) ·
[Backstage createDevApp](https://backstage.io/api/next/functions/_backstage_frontend-dev-utils.createDevApp.html) ·
[Consumer-Driven Contracts](https://martinfowler.com/articles/consumerDrivenContracts.html) ·
[Software Architecture Patterns, ch. 3](https://www.oreilly.com/library/view/software-architecture-patterns/9781491971437/ch03.html)
