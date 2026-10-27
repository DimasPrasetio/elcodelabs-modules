# ADR 0002 — The library documents itself, and the checks keep it current

**Status:** accepted, 27 October 2026.

## Context

ADR 0001 made the library buildable and testable with this repository alone. Knowing *where
things stand* still lived in the maintainer's head and in commit messages: what is released and
tested, what is in progress, what comes next, what is knowingly limited. A developer or an AI
agent given only this repository had to reconstruct it. Hand-written status documents go stale
— the README's own list of testbed scenarios had already fallen two scenarios behind.

## Decision

1. **Three documents, one home per fact.**
   - [STATUS.md](../../STATUS.md): where the library stands — a generated snapshot (modules,
     versions, kernel, testbed coverage, scenarios) and hand-written Last verified, In progress,
     Next, Known limits, and Log.
   - [docs/BUILDING_MODULES.md](../BUILDING_MODULES.md): how to build a module, from idea to
     release, with the patterns and pitfalls learned so far.
   - [docs/plans/](../plans/README.md): the design and results of each larger module.

   AGENTS.md keeps the rules; MODULE_FORMAT.md the manifest; CATALOG.md the generated choice list.
2. **Generated where it can be.** `scripts/catalog.mjs` writes STATUS.md's snapshot between its
   markers, and `--check` fails when it is stale — like CATALOG.md. The testbed scenarios are
   described once, in the header of `scripts/testbed.mjs`, and listed from there.
3. **Checked where it cannot.**
   - `status-current` (with `--since`, so the pre-commit hook and CI): a change that releases a
     module or preset, moves the kernel, or changes the rule set or the testbed scenarios must
     also change STATUS.md's hand-written part, and name each released module with its version.
   - `testbed-covered`: every module is installed by a scenario.
   - `testbed-documented`: every scenario is described.
   - `rules-documented`: every rule a check reports is in AGENTS.md's table.
4. **Work happens on branches; a release is a merge to `main`.** On a branch the hook checks the
   branch as one change against `main` and lets `testbed-covered` and `status-current` warn; the
   pull request's check enforces them, and only `main` is tagged. Without that, every commit was
   a release and a module could not be built over several commits.
5. **STATUS.md is open to module developers**; plans, guides, and rules stay under the
   maintainer's review (`.github/CODEOWNERS`) — the review of a plan's pull request is its
   approval.

## Consequences

- A release commit carries its own status update; the log is the human history beside `git log`.
- The checks cannot judge whether the words are *true* — only that they moved with the change and
  name what was released. Reviews do the rest. Known gaps: a change to a rule's logic (not its
  name), to a scenario's checks (not the scenario list), or to the tools (`scripts/`, guides)
  does not by itself ask for a STATUS.md line — the code-owner review of those paths does.
