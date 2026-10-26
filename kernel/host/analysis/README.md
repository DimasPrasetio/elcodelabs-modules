# Analysis

Per-requirement analysis artifacts, one folder per requirement:
`analysis/<NNN>-<slug>/` or `analysis/<ISSUE-ID>-<slug>/` if there's an issue tracker.

Each folder contains: `analysis.md`, `flow.mmd`, `sequence.mmd`, and optionally `erd.mmd`.

This folder is committed on purpose — it's the record of why something was built the way it
was, for future-you and for the next Claude session. Do not gitignore it.

Artifacts here are never edited after being written. If understanding changes, write a new
analysis — don't rewrite the old one.
